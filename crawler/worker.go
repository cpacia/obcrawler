package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cpacia/obcrawler/repo"
	core2 "github.com/cpacia/openbazaar3.0/core"
	"github.com/cpacia/openbazaar3.0/core/coreiface"
	"github.com/cpacia/openbazaar3.0/models"
	"github.com/cpacia/openbazaar3.0/orders/pb"
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

type Job struct {
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
			r := rand.Intn(len(c.nodes))

			// If the job was passed in with a nil record it means the caller wants us to try to
			// fetch and/or refresh the record so we will try to get the record from routing. This
			// will fail if the record is expired or unavailable.
			if job.IPNSRecord == nil {
				rec, err := fetchIPNSRecord(c.ctx, c.nodes[r].IPFSNode(), job.Peer, int(c.ipnsQuorum))
				if err != nil {
					log.Warningf("IPNS record not found for peer %s", job.Peer.Pretty())
					continue
				}
				job.IPNSRecord = rec
				eol, err := ipns.GetEOL(rec)
				if err != nil {
					log.Warningf("Error unmarshalling record eol for peer %s", job.Peer.Pretty())
					continue
				}
				job.Expiration = eol
				if bytes.Equal(rec.GetValue(), job.LastKnownVal) {
					continue
				}
			}

			// Next we want to load the IPLD node for the record's root CID. We can use this
			// to get the CIDs of the profile and listing index and determine if we need to
			// download them.
			rootCID, err := cid.Decode(string(job.IPNSRecord.GetValue()))
			if err != nil {
				log.Warningf("Error unmarshalling record cid for peer %s: %s", job.Peer.Pretty(), err)
				continue
			}

			nd, err := c.dagGet(c.ctx, c.nodes[r].IPFSNode(), rootCID)
			if err != nil {
				log.Warningf("Error fetching root node for peer %s: %s", job.Peer.Pretty(), err)
				continue
			}

			profileLink, _, err := nd.ResolveLink([]string{"profile.json"})
			if err != nil && err != merkledag.ErrLinkNotFound {
				log.Warningf("Error resolving profile link for peer %s: %s", job.Peer.Pretty(), err)
				continue
			}
			listingsLink, _, err := nd.ResolveLink([]string{"listings.json"})
			if err != nil && err != merkledag.ErrLinkNotFound {
				log.Warningf("Error resolving listings link for peer %s: %s", job.Peer.Pretty(), err)
				continue
			}

			// Check the db to see if we've already crawled these CIDs.
			needProfile, needListings := false, false
			err = c.db.View(func(db *gorm.DB) error {
				if profileLink != nil {
					err := db.Where("c_id=?", profileLink.Cid.String()).First(&repo.CIDRecord{}).Error
					if err != nil && !gorm.IsRecordNotFoundError(err) {
						return err
					} else if gorm.IsRecordNotFoundError(err) {
						needProfile = true
					}
				}
				if listingsLink != nil {
					err := db.Where("c_id=?", listingsLink.Cid.String()).First(&repo.CIDRecord{}).Error
					if err != nil && !gorm.IsRecordNotFoundError(err) {
						return err
					} else if gorm.IsRecordNotFoundError(err) {
						needListings = true
					}
				}
				return nil
			})
			if err != nil {
				log.Warningf("Error querying cid db for peer %s: %s", job.Peer.Pretty(), err)
				continue
			}

			if needProfile {
				profileBytes, err := c.cat(c.ctx, c.nodes[r].IPFSNode(), path.IpfsPath(profileLink.Cid))
				if err == nil {
					var profile models.Profile
					err := json.Unmarshal(profileBytes, &profile)
					if err == nil {
						log.Debugf("Crawled profile for peer %s", job.Peer.Pretty())
						c.subMtx.RLock()
						for _, sub := range c.subs {
							sub.Out <- &Object{
								ExpirationDate: job.Expiration,
								Data:           &profile,
							}
						}
						c.subMtx.RUnlock()
						if c.cacheImages {
							go c.downloadImages(c.ctx, c.nodes[r], &profile)
						}
					}
				}
			}

			var newListings []string
			if needListings {
				listingBytes, err := c.cat(c.ctx, c.nodes[r].IPFSNode(), path.IpfsPath(listingsLink.Cid))
				if err == nil {
					var listingIndex models.ListingIndex
					err := json.Unmarshal(listingBytes, &listingIndex)
					if err == nil {
						log.Debugf("Crawled listing index for peer %s", job.Peer.Pretty())

						toDownload := make([]string, 0, len(listingIndex))
						err = c.db.View(func(db *gorm.DB) error {
							for _, listing := range listingIndex {
								err := db.Where("c_id=?", listing.CID).First(&repo.CIDRecord{}).Error
								if err != nil && !gorm.IsRecordNotFoundError(err) {
									return err
								} else if gorm.IsRecordNotFoundError(err) {
									toDownload = append(toDownload, listing.CID)
								}
							}
							return nil
						})
						if err != nil {
							log.Warningf("Error querying cid db for peer %s: %s", job.Peer.Pretty(), err)
							continue
						}
						newListings = toDownload

						for _, cidStr := range toDownload {
							id, err := cid.Decode(cidStr)
							if err != nil {
								log.Errorf("Error decoding CID: %s", err)
								continue
							}
							listing, err := c.nodes[r].GetListingByCID(c.ctx, id)
							if err != nil {
								log.Errorf("Unable to load listing %s for peer %s: %s", id.String(), job.Peer.Pretty(), err)
								continue
							}
							log.Debugf("Crawled listing %s for peer %s", listing.Cid, job.Peer.Pretty())
							c.subMtx.RLock()
							for _, sub := range c.subs {
								sub.Out <- &Object{
									ExpirationDate: job.Expiration,
									Data:           listing,
								}
							}
							c.subMtx.RUnlock()
							if c.cacheImages {
								go c.downloadImages(c.ctx, c.nodes[r], listing)
							}
						}
					}
				}
			}

			if needProfile || needListings {
				err = c.db.Update(func(db *gorm.DB) error {
					if needProfile && profileLink != nil {
						db.Save(&repo.CIDRecord{
							CID:    profileLink.Cid.String(),
							PeerID: job.Peer.Pretty(),
						})
					}
					if needListings && listingsLink != nil {
						db.Save(&repo.CIDRecord{
							CID:    listingsLink.Cid.String(),
							PeerID: job.Peer.Pretty(),
						})
					}
					for _, id := range newListings {
						db.Save(&repo.CIDRecord{
							CID:    id,
							PeerID: job.Peer.Pretty(),
						})
					}
					return nil
				})
				if err != nil {
					log.Warningf("Error saving new cids for peer %s: %s", job.Peer.Pretty(), err)
					continue
				}
			}
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

func (c *Crawler) downloadImages(ctx context.Context, n *core2.OpenBazaarNode, obj interface{}) {
	var toDownload []string
	switch o := obj.(type) {
	case *models.Profile:
		err := c.db.View(func(db *gorm.DB) error {
			ids := []string{o.AvatarHashes.Tiny, o.AvatarHashes.Small, o.AvatarHashes.Medium, o.AvatarHashes.Large, o.AvatarHashes.Original, o.HeaderHashes.Tiny, o.HeaderHashes.Small, o.HeaderHashes.Medium, o.HeaderHashes.Large, o.HeaderHashes.Original}
			for _, id := range ids {
				err := db.Where("c_id=?", id).First(&repo.CIDRecord{}).Error
				if err != nil && !gorm.IsRecordNotFoundError(err) {
					return err
				} else if gorm.IsRecordNotFoundError(err) {
					toDownload = append(toDownload, id)
				}
			}
			return nil
		})
		if err != nil {
			log.Warningf("Error querying cid db %s", err)
			return
		}
	case *pb.SignedListing:
		err := c.db.View(func(db *gorm.DB) error {
			for _, image := range o.Listing.Item.Images {
				ids := []string{image.Tiny, image.Small, image.Medium, image.Large, image.Original}
				for _, id := range ids {
					err := db.Where("c_id=?", id).First(&repo.CIDRecord{}).Error
					if err != nil && !gorm.IsRecordNotFoundError(err) {
						return err
					} else if gorm.IsRecordNotFoundError(err) {
						toDownload = append(toDownload, id)
					}
				}
			}
			return nil
		})
		if err != nil {
			log.Warningf("Error querying cid db %s", err)
			return
		}
	}
	for _, cidStr := range toDownload {
		id, err := cid.Decode(cidStr)
		if err != nil {
			continue
		}
		_, err = n.GetImage(ctx, id)
		if err == nil {
			log.Infof("Cached image %s", cidStr)
		}
	}
}
