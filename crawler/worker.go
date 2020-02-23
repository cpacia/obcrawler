package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cpacia/openbazaar3.0/core/coreiface"
	"github.com/cpacia/openbazaar3.0/models"
	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipns"
	ipnspb "github.com/ipfs/go-ipns/pb"
	"github.com/ipfs/interface-go-ipfs-core/path"
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

			p := path.Join(path.New(string(job.IPNSRecord.GetValue())), "profile.json")
			profileBytes, err := c.cat(c.ctx, c.nodes[r].IPFSNode(), p)
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
				}
			}

			p = path.Join(path.New(string(job.IPNSRecord.GetValue())), "listings.json")
			listingBytes, err := c.cat(c.ctx, c.nodes[r].IPFSNode(), p)
			if err == nil {
				var listingIndex models.ListingIndex
				err := json.Unmarshal(listingBytes, &listingIndex)
				if err == nil {
					log.Debugf("Crawled listing index for peer %s", job.Peer.Pretty())
					for _, listing := range listingIndex {
						id, err := cid.Decode(listing.CID)
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
					}
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
