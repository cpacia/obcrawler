package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cpacia/obcrawler/repo"
	"github.com/cpacia/obcrawler/rpc"
	"github.com/cpacia/openbazaar3.0/core/coreiface"
	"github.com/cpacia/openbazaar3.0/models"
	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-ipns"
	ipnspb "github.com/ipfs/go-ipns/pb"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/jinzhu/gorm"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	peer "github.com/libp2p/go-libp2p-peer"
	"io/ioutil"
	"math/rand"
	"time"
)

const catTimeout = time.Second * 30

type job struct {
	Peer         peer.ID
	IPNSRecord   *ipnspb.IpnsEntry
	Expiration   time.Time
	LastKnownVal []byte
}

func (c *Crawler) worker() {
	for {
		select {
		case <-c.shutdown:
			return
		case job := <-c.workChan:
			// We're wrapping in a function here so we can defer update the last crawled time.
			// The time gets updated regardless of whether the crawl succeeds or not so we don't
			// repeat the crawls over and over if there's a failure.
			func() {
				log.Debugf("Starting crawl of peer %s", job.Peer.Pretty())
				start := time.Now()
				defer func() {
					err := c.db.Update(func(db *gorm.DB) error {
						var peer repo.Peer
						err := db.Where("peer_id=?", job.Peer.Pretty()).First(&peer).Error
						if err != nil && !gorm.IsRecordNotFoundError(err) {
							return err
						}
						peer.LastCrawled = time.Now()
						return db.Save(&peer).Error
					})
					if err != nil {
						log.Errorf("Error saving last crawled time for peer %s: %s", job.Peer.Pretty(), err)
					}
					log.Debugf("Crawl of %s finished in %s", job.Peer.Pretty(), time.Since(start))
				}()

				// We'll pick one random node to use for our crawls.
				r := rand.Intn(len(c.nodes))

				// If the job was passed in with a nil record it means the caller wants us to try to
				// fetch and/or refresh the record so we will try to get the record from routing. This
				// will fail if the record is expired or unavailable.
				if job.IPNSRecord == nil {
					rec, err := fetchIPNSRecord(c.ctx, c.nodes[r].IPFSNode(), job.Peer, int(c.ipnsQuorum))
					if err != nil {
						log.Warningf("IPNS record not found for peer %s", job.Peer.Pretty())
						return
					}
					job.IPNSRecord = rec
					eol, err := ipns.GetEOL(rec)
					if err != nil {
						log.Warningf("Error unmarshalling record eol for peer %s", job.Peer.Pretty())
						return
					}
					job.Expiration = eol
					if bytes.Equal(rec.GetValue(), job.LastKnownVal) {
						log.Debugf("IPNS record for peer %s is unchanged", job.Peer.Pretty())
						return
					}
				}

				// Next we want to load the IPLD node for the record's root CID. We can use this
				// to get the CIDs of the profile, listing index, and rating index and determine if
				// we need to download them.
				rootCID, err := cid.Decode(string(job.IPNSRecord.GetValue()))
				if err != nil {
					log.Warningf("Error unmarshalling record cid for peer %s: %s", job.Peer.Pretty(), err)
					return
				}

				nd, err := c.dagGet(c.ctx, c.nodes[r].IPFSNode(), rootCID)
				if err != nil {
					log.Warningf("Error fetching root node for peer %s: %s", job.Peer.Pretty(), err)
					return
				}

				profileLink, _, err := nd.ResolveLink([]string{"profile.json"})
				if err != nil && err != merkledag.ErrLinkNotFound {
					log.Warningf("Error resolving profile link for peer %s: %s", job.Peer.Pretty(), err)
					return
				}
				listingsLink, _, err := nd.ResolveLink([]string{"listings.json"})
				if err != nil && err != merkledag.ErrLinkNotFound {
					log.Warningf("Error resolving listings link for peer %s: %s", job.Peer.Pretty(), err)
					return
				}
				ratingsLink, _, err := nd.ResolveLink([]string{"ratings.json"})
				if err != nil && err != merkledag.ErrLinkNotFound {
					log.Warningf("Error resolving ratings link for peer %s: %s", job.Peer.Pretty(), err)
				}

				// If the profile link exists, crawl the profile.
				var images []string
				if profileLink != nil {
					profileBytes, err := c.cat(c.ctx, c.nodes[r].IPFSNode(), path.IpfsPath(profileLink.Cid))
					if err == nil {
						var profile models.Profile
						err := json.Unmarshal(profileBytes, &profile)
						if err == nil {
							log.Debugf("Crawled profile for peer %s", job.Peer.Pretty())

							// Send the found profile to subscribers.
							c.notifySubscribers(&rpc.Object{
								ExpirationDate: job.Expiration,
								Data:           &profile,
							})

							// If we're caching data then add the image hashes to the image slice.
							if c.cacheData {
								images = append(images,
									profile.AvatarHashes.Original,
									profile.AvatarHashes.Large,
									profile.AvatarHashes.Medium,
									profile.AvatarHashes.Small,
									profile.AvatarHashes.Tiny,
									profile.HeaderHashes.Original,
									profile.HeaderHashes.Large,
									profile.HeaderHashes.Medium,
									profile.HeaderHashes.Small,
									profile.HeaderHashes.Tiny)
							}
						}
					}
				}

				// If the listing index link exists, crawl the listings.
				var newListings []string
				if listingsLink != nil {
					listingBytes, err := c.cat(c.ctx, c.nodes[r].IPFSNode(), path.IpfsPath(listingsLink.Cid))
					if err == nil {
						var listingIndex models.ListingIndex
						err := json.Unmarshal(listingBytes, &listingIndex)
						if err == nil {
							log.Debugf("Crawled listing index for peer %s", job.Peer.Pretty())
							// Now that we have the index, range over each listing and try to download it.
							for _, listing := range listingIndex {
								id, err := cid.Decode(listing.CID)
								if err != nil {
									log.Errorf("Error decoding CID: %s", err)
									continue
								}
								newListings = append(newListings, listing.CID)
								listing, err := c.nodes[r].GetListingByCID(c.ctx, id)
								if err != nil {
									log.Errorf("Unable to load listing %s for peer %s: %s", id.String(), job.Peer.Pretty(), err)
									continue
								}
								log.Debugf("Crawled listing %s for peer %s", listing.Cid, job.Peer.Pretty())

								// Send the found listing to the subscribers.
								c.notifySubscribers(&rpc.Object{
									ExpirationDate: job.Expiration,
									Data:           listing,
								})

								// If we're caching data then add the image hashes to the image slice.
								if c.cacheData {
									for _, image := range listing.Listing.Item.Images {
										images = append(images,
											image.Original,
											image.Large,
											image.Medium,
											image.Small,
											image.Tiny)
									}
								}
							}
						}
					}
				}

				// If the rating link exists, crawl the ratings. Note that we don't
				// index ratings. This is purely for caching purposes to make sure
				// they remain available on the network.
				var ratings []string
				if ratingsLink != nil && c.cacheData {
					ratingsBytes, err := c.cat(c.ctx, c.nodes[r].IPFSNode(), path.IpfsPath(ratingsLink.Cid))
					if err == nil {
						var ratingIndex models.RatingIndex
						err := json.Unmarshal(ratingsBytes, &ratingIndex)
						if err == nil {
							log.Debugf("Crawled rating index for peer %s", job.Peer.Pretty())

							// At the rating CID to the ratings slice.
							for _, item := range ratingIndex {
								ratings = append(ratings, item.Ratings...)
							}
						}
					}
				}

				// Finally we want to:
				// 1) Load all existing CIDs for this peer.
				// 2) Find the diff between the existing CIDs and new CIDs.
				// 3) For each new CID, pin it (this will download from IPFS if necessary).
				// 4) Unpin any CIDs that are not carrying forward. This will make them available
				// to be garbage collected.
				// 5) Delete CIDs not carrying forward from the db.
				var (
					oldCIDs []repo.CIDRecord
					newCIDs = make(map[string]bool)
				)
				err = c.db.Update(func(db *gorm.DB) error {
					err := db.Where("peer_id=?", job.Peer.Pretty()).Find(&oldCIDs).Error
					if err != nil && !gorm.IsRecordNotFoundError(err) {
						log.Errorf("Error loading current CIDs from DB for peer %s: %s", job.Peer.Pretty(), err)
					}

					// Root CID is added to the new map.
					if err := db.Save(&repo.CIDRecord{
						CID:    rootCID.String(),
						PeerID: job.Peer.Pretty(),
					}).Error; err != nil {
						return err
					}
					newCIDs[rootCID.String()] = true

					// Profile CID is added to the new map.
					if profileLink != nil {
						if err := db.Save(&repo.CIDRecord{
							CID:    profileLink.Cid.String(),
							PeerID: job.Peer.Pretty(),
						}).Error; err != nil {
							return err
						}
						newCIDs[profileLink.Cid.String()] = true
					}
					// Listing Index CID is added to the new map.
					if listingsLink != nil {
						if err := db.Save(&repo.CIDRecord{
							CID:    listingsLink.Cid.String(),
							PeerID: job.Peer.Pretty(),
						}).Error; err != nil {
							return err
						}
						newCIDs[listingsLink.Cid.String()] = true
					}
					// Each listing CID is added to the new map.
					for _, id := range newListings {
						if err := db.Save(&repo.CIDRecord{
							CID:    id,
							PeerID: job.Peer.Pretty(),
						}).Error; err != nil {
							return err
						}
						newCIDs[id] = true
					}
					// Each image CID is added to the new map.
					for _, id := range images {
						if _, err := cid.Decode(id); err != nil {
							continue
						}
						if err := db.Save(&repo.CIDRecord{
							CID:    id,
							PeerID: job.Peer.Pretty(),
						}).Error; err != nil {
							return err
						}
						newCIDs[id] = true
					}
					// Rating Index CID is added to the new map.
					if ratingsLink != nil {
						if err := db.Save(&repo.CIDRecord{
							CID:    ratingsLink.Cid.String(),
							PeerID: job.Peer.Pretty(),
						}).Error; err != nil {
							return err
						}
						newCIDs[ratingsLink.Cid.String()] = true
					}
					// Each rating CID is added to the new map.
					for _, id := range ratings {
						if _, err := cid.Decode(id); err != nil {
							continue
						}
						if err := db.Save(&repo.CIDRecord{
							CID:    id,
							PeerID: job.Peer.Pretty(),
						}).Error; err != nil {
							return err
						}
						newCIDs[id] = true
					}
					return nil
				})
				if err != nil {
					log.Warningf("Error saving new cids for peer %s: %s", job.Peer.Pretty(), err)
					return
				}

				capi, err := coreapi.NewCoreAPI(c.nodes[r].IPFSNode())
				if err != nil {
					log.Warningf("Error loading core API during crawl of %s: %s", job.Peer.Pretty(), err)
					return
				}

				// Pin all new files.
				if c.pinFiles {
					for idStr := range newCIDs {
						ctx, cancel := context.WithTimeout(c.ctx, catTimeout)
						defer cancel()
						id, err := cid.Decode(idStr)
						if err != nil {
							log.Errorf("Error decoding CID for pinning, peer %s: %s", job.Peer.Pretty(), err)
							continue
						}
						// If we already have this file this should just return immediately.
						// Otherwise it will be downloaded and pinned.
						if err := capi.Pin().Add(ctx, path.IpfsPath(id)); err != nil {
							log.Errorf("Error pinning file %s for peer %s: %s", id, job.Peer.Pretty(), err)
						}
					}
				}

				// If the old CID is not carrying forward, unpin and delete from the db.
				var toDelete []string
				for _, rec := range oldCIDs {
					if !newCIDs[rec.CID] {
						id, err := cid.Decode(rec.CID)
						if err != nil {
							log.Errorf("Error decoding CID for unpinning, peer %s: %s", job.Peer.Pretty(), err)
							continue
						}
						err = c.unpinCID(id)
						if err != nil {
							log.Errorf("Error unpinning file %s for peer %s: %s", id, job.Peer.Pretty(), err)
						} else {
							toDelete = append(toDelete, rec.CID)
						}
					}
				}
				c.db.Update(func(db *gorm.DB) error {
					for _, id := range toDelete {
						err = db.Where("c_id = ?", id).Delete(&repo.CIDRecord{}).Error
						if err != nil {
							log.Errorf("Error deleting CID record for peer %s: %s", job.Peer.Pretty(), err)
						}
					}
					return nil
				})
			}()
		}
	}
}

func fetchIPNSRecord(ctx context.Context, n *core.IpfsNode, pid peer.ID, ipnsQuorum int) (*ipnspb.IpnsEntry, error) {
	// Use the routing system to get the name.
	// Note that the DHT will call the ipns validator when retrieving
	// the value, which in turn verifies the ipns record signature
	ipnsKey := ipns.RecordKey(pid)

	val, err := n.Routing.GetValue(ctx, ipnsKey, dht.Quorum(ipnsQuorum))
	if err != nil {
		return nil, err
	}

	rec := new(ipnspb.IpnsEntry)
	if err := proto.Unmarshal(val, rec); err != nil {
		return nil, err
	}

	pubkey, err := pid.ExtractPublicKey()
	if err != nil {
		return nil, err
	}

	if err := ipns.Validate(pubkey, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (c *Crawler) cat(ctx context.Context, n *core.IpfsNode, pth path.Path) ([]byte, error) {
	catDone := make(chan struct{})
	ctx, cancel := context.WithTimeout(ctx, catTimeout)
	defer func() {
		cancel()
		catDone <- struct{}{}
	}()

	capi, err := coreapi.NewCoreAPI(n)
	if err != nil {
		return nil, err
	}

	go func() {
		select {
		case <-catDone:
			return
		case <-c.shutdown:
			cancel()
		}
	}()

	nd, err := capi.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
	}

	r, ok := nd.(files.File)
	if !ok {
		return nil, errors.New("incorrect type from Unixfs().Get()")
	}

	return ioutil.ReadAll(r)
}

func (c *Crawler) dagGet(ctx context.Context, n *core.IpfsNode, cid cid.Cid) (ipld.Node, error) {
	catDone := make(chan struct{})
	ctx, cancel := context.WithTimeout(ctx, catTimeout)
	defer func() {
		cancel()
		catDone <- struct{}{}
	}()

	capi, err := coreapi.NewCoreAPI(n)
	if err != nil {
		return nil, err
	}

	go func() {
		select {
		case <-catDone:
			return
		case <-c.shutdown:
			cancel()
		}
	}()

	nd, err := capi.Dag().Get(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", coreiface.ErrNotFound, err)
	}

	return nd, nil
}
