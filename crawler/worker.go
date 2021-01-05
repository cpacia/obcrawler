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
	"github.com/ipfs/go-ipfs/namesys"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-ipns"
	ipnspb "github.com/ipfs/go-ipns/pb"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"gorm.io/gorm"
	"io/ioutil"
	"math/rand"
	"time"
)

const catTimeout = time.Second * 30

type job struct {
	Peer           peer.ID
	IPNSRecord     *ipnspb.IpnsEntry
	Expiration     time.Time
	FetchNewRecord bool
	PinRecord      bool
}

func (c *Crawler) worker() {
	for {
		select {
		case <-c.shutdown:
			return
		case job := <-c.workChan:
			c.processJob(job)
		}
	}
}

func (c *Crawler) processJob(job *job) {
	log.Debugf("Starting crawl of peer %s", job.Peer.Pretty())
	start := time.Now()

	// We'll pick one random node to use for our crawls.
	r := rand.Intn(len(c.nodes))

	// We are going to defer update the LastCrawled time regardless of whether the crawl
	// succeeds or not so that we don't get stuck in a loop perpetually crawling nodes
	// which errored.
	defer func() {
		// Pin IPNS record if requested.
		if job.PinRecord && job.IPNSRecord != nil {
			if err := namesys.PutRecordToRouting(c.ctx, c.nodes[r].IPFSNode().Routing, nil, job.IPNSRecord); err != nil {
				log.Errorf("Error pinning IPNS record for peer %s: %s", job.Peer.Pretty(), err)
			}
		}
		err := c.db.Update(func(db *gorm.DB) error {
			var peer repo.Peer
			err := db.Where("peer_id=?", job.Peer.Pretty()).First(&peer).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			peer.LastCrawled = time.Now()
			if job.PinRecord {
				peer.LastPinned = time.Now()
			}
			return db.Save(&peer).Error
		})
		if err != nil {
			log.Errorf("Error saving last crawled time for peer %s: %s", job.Peer.Pretty(), err)
		}
		log.Debugf("Crawl of %s finished in %s", job.Peer.Pretty(), time.Since(start))
	}()

	// If FetchNewRecord is true it means the caller wants us to try to fetch and/or refresh
	// the record so we will try to get the record from routing. This will fail if the record
	// is expired or unavailable.
	if job.FetchNewRecord || job.IPNSRecord == nil {
		rec, err := fetchIPNSRecord(c.ctx, c.nodes[r].IPFSNode(), job.Peer, int(c.ipnsQuorum))
		if err != nil {
			log.Warningf("IPNS record not found for peer %s", job.Peer.Pretty())
			return
		}

		if job.IPNSRecord != nil && bytes.Equal(rec.GetValue(), job.IPNSRecord.Value) {
			log.Debugf("IPNS record for peer %s is unchanged", job.Peer.Pretty())
			return
		}

		job.IPNSRecord = rec
		eol, err := ipns.GetEOL(rec)
		if err != nil {
			log.Warningf("Error unmarshalling record eol for peer %s", job.Peer.Pretty())
			return
		}
		job.Expiration = eol
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

	// If the profile link exists, crawl the profile.
	if profileLink != nil {
		profileBytes, err := c.cat(c.ctx, c.nodes[r].IPFSNode(), path.IpfsPath(profileLink.Cid))
		if err == nil {
			var profile models.Profile
			err := json.Unmarshal(profileBytes, &profile)
			if err == nil {
				log.Debugf("Crawled profile for peer %s", job.Peer.Pretty())

				// Send the found profile to subscribers.
				defer c.notifySubscribers(&rpc.Object{
					ExpirationDate: job.Expiration,
					Data:           &profile,
				})
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
						log.Errorf("Error decoding CID for peer %s: %s", job.Peer.Pretty(), err)
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
					defer c.notifySubscribers(&rpc.Object{
						ExpirationDate: job.Expiration,
						Data:           listing,
					})
				}
			}
		}
	}

	// If cacheData is set then we will traverse the full graph for this node and
	// download all cids under the root so that we can pin them.
	var graph []cid.Cid
	if c.cacheData {
		graph, err = c.fetchGraph(c.nodes[r].IPFSNode(), &rootCID)
		if err != nil {
			log.Errorf("Error fetching graph for peer %s: %s", job.Peer.Pretty(), err)
		}
	}
	graph = append(graph, rootCID)
	if profileLink != nil {
		graph = append(graph, profileLink.Cid)
	}
	if listingsLink != nil {
		graph = append(graph, listingsLink.Cid)
	}
	for _, l := range newListings {
		id, err := cid.Decode(l)
		if err != nil {
			continue
		}
		graph = append(graph, id)
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
		toUnpin []string
	)
	err = c.db.Update(func(db *gorm.DB) error {
		err := db.Where("peer_id=?", job.Peer.Pretty()).Find(&oldCIDs).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("Error loading current CIDs from DB for peer %s: %s", job.Peer.Pretty(), err)
		}

		for _, id := range graph {
			if !newCIDs[id.String()] {
				if err := db.Save(&repo.CIDRecord{
					CID:    id.String(),
					PeerID: job.Peer.Pretty(),
				}).Error; err != nil {
					return err
				}
				newCIDs[id.String()] = true
			}
		}

		for _, rec := range oldCIDs {
			if !newCIDs[rec.CID] {
				// If the old CID is not in the new list then delete it from the db for this peer.
				err = db.Where("c_id = ?", rec.CID).Where("peer_id=?", job.Peer.Pretty()).Delete(&repo.CIDRecord{}).Error
				if err != nil {
					log.Errorf("Error deleting CID record for peer %s: %s", job.Peer.Pretty(), err)
				}

				// While we deleted it for this peer, we only want to unpin it if no other peer has this CID.
				// So here we will query the db to see if we can find any stored by any other peers. If no, then
				// we will add it to our unpin list.
				var recs []repo.CIDRecord
				err = db.Where("c_id=?", rec.CID).Where("peer_id<>?", job.Peer.Pretty()).Find(&recs).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				if len(recs) == 0 {
					toUnpin = append(toUnpin, rec.CID)
				}
			}
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
		ctx, cancel := context.WithTimeout(c.ctx, catTimeout)
		defer cancel()

		// If we already have all the files this should just return immediately.
		// Otherwise missing files will be downloaded and pinned.
		if err := capi.Pin().Add(ctx, path.IpfsPath(rootCID)); err != nil {
			log.Errorf("Error recursively pinning rootCID %s for peer %s: %s", rootCID.String(), job.Peer.Pretty(), err)
		}
	}

	// If the old CID is not carrying forward, unpin.
	for _, u := range toUnpin {
		id, err := cid.Decode(u)
		if err != nil {
			log.Errorf("Error decoding CID for unpinning, peer %s: %s", job.Peer.Pretty(), err)
			continue
		}
		err = c.unpinCID(id)
		if err != nil {
			log.Errorf("Error unpinning file %s for peer %s: %s", id, job.Peer.Pretty(), err)
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

func (c *Crawler) fetchGraph(n *core.IpfsNode, id *cid.Cid) ([]cid.Cid, error) {
	var (
		ret []cid.Cid
		m   = make(map[string]bool)
	)
	m[id.String()] = true
	for {
		if len(m) == 0 {
			break
		}
		for k := range m {
			i, err := cid.Decode(k)
			if err != nil {
				return ret, err
			}
			ret = append(ret, i)
			nd, err := c.dagGet(c.ctx, n, i)
			if err != nil {
				return ret, err
			}
			delete(m, k)
			for _, link := range nd.Links() {
				m[link.Cid.String()] = true
			}
		}
	}
	return ret, nil
}
