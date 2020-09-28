package crawler

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/cpacia/obcrawler/repo"
	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipns"
	ipnspb "github.com/ipfs/go-ipns/pb"
	iface "github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/jinzhu/gorm"
	"sync"
	"time"
)

const ipnsPubsubTopic = "/ipns/all"

func (c *Crawler) listenPubsub() error {
	messageChan := make(chan iface.PubSubMessage)

	subs := make([]iface.PubSubSubscription, c.numPubsub)
	for _, n := range c.nodes[:c.numPubsub] {
		api, err := coreapi.NewCoreAPI(n.IPFSNode())
		if err != nil {
			return err
		}
		sub, err := api.PubSub().Subscribe(c.ctx, ipnsPubsubTopic, caopts.PubSub.Discover(true))
		if err != nil {
			return err
		}
		subs = append(subs, sub)

		go func() {
			for {
				message, err := sub.Next(c.ctx)
				if err != nil {
					log.Errorf("Error fetching next subscription message %s", err.Error())
					return
				}
				messageChan <- message
			}
		}()
	}
	go func() {
		mtx := sync.Mutex{}
		recentMessasges := make(map[string]struct{})

		for {
			select {
			case <-c.shutdown:
				for _, sub := range subs {
					sub.Close()
				}
				return
			case message := <-messageChan:
				h := sha256.Sum256(append([]byte(message.From()), message.Data()...))
				id := hex.EncodeToString(h[:])

				mtx.Lock()
				_, ok := recentMessasges[id]
				if ok {
					mtx.Unlock()
					continue
				}
				recentMessasges[id] = struct{}{}
				mtx.Unlock()

				time.AfterFunc(time.Minute, func() {
					mtx.Lock()
					delete(recentMessasges, id)
					mtx.Unlock()
				})

				rec := new(ipnspb.IpnsEntry)
				if err := proto.Unmarshal(message.Data(), rec); err != nil {
					log.Errorf("Error unmarshalling IPNS record for peer %s: %s", message.From().Pretty(), err)
					continue
				}

				pubkey, err := message.From().ExtractPublicKey()
				if err != nil {
					log.Errorf("Error extracting public key for %s: %s", message.From().Pretty(), err)
					continue
				}

				if err := ipns.Validate(pubkey, rec); err != nil {
					log.Errorf("Received invalid IPNS record for %s: %s", message.From().Pretty(), err)
					continue
				}

				expiration, err := ipns.GetEOL(rec)
				if err != nil {
					log.Errorf("Error extracting IPNS record eol for %s: %s", message.From().Pretty(), err)
					continue
				}

				banned := false
				err = c.db.Update(func(db *gorm.DB) error {
					var peer repo.Peer
					err := db.Where("peer_id=?", message.From().Pretty()).First(&peer).Error
					if err != nil && !gorm.IsRecordNotFoundError(err) {
						return err
					} else if gorm.IsRecordNotFoundError(err) {
						peer.PeerID = message.From().Pretty()
						peer.FirstSeen = time.Now()
						peer.LastSeen = time.Now()
						log.Infof("Detected new peer: %s", message.From().Pretty())
					}
					peer.IPNSExpiration = expiration
					peer.IPNSRecord = message.Data()
					banned = peer.Banned
					return db.Save(&peer).Error
				})
				if err != nil {
					log.Errorf("Error saving IPNS record for peer %s: %s", message.From().Pretty(), err)
				}
				if banned {
					continue
				}
				log.Debugf("Received new IPNS record from %s. Expiration %s", message.From().Pretty(), expiration)
				go func() {
					c.workChan <- &job{
						Peer:       message.From(),
						Expiration: expiration,
						IPNSRecord: rec,
					}
				}()
			}
		}
	}()
	return nil
}
