package crawler

import (
	"github.com/cpacia/obcrawler/repo"
	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipns"
	ipnspb "github.com/ipfs/go-ipns/pb"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/jinzhu/gorm"
	"time"
)

const ipnsPubsubTopic = "/ipns/all"

func (c *Crawler) listenPubsub() error {
	api, err := coreapi.NewCoreAPI(c.nodes[len(c.nodes)-1].IPFSNode())
	if err != nil {
		return err
	}
	sub, err := api.PubSub().Subscribe(c.ctx, ipnsPubsubTopic)
	if err != nil {
		return err
	}

	messageChan := make(chan iface.PubSubMessage)
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
	go func() {
		for {
			select {
			case <-c.shutdown:
				return
			case message := <-messageChan:
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

				err = c.db.Update(func(db *gorm.DB) error {
					var peer repo.Peer
					err := db.Where("peer_id=?", message.From().Pretty()).First(&peer).Error
					if err != nil && !gorm.IsRecordNotFoundError(err) {
						return err
					} else if gorm.IsRecordNotFoundError(err) {
						peer.PeerID = message.From().Pretty()
						peer.FirstSeen = time.Now()
						peer.LastSeen = time.Now()
					}
					peer.IPNSExpiration = expiration
					peer.IPNSRecord = message.Data()
					return db.Save(&peer).Error
				})
				if err != nil {
					log.Errorf("Error saving IPNS record for peer %s: %s", message.From().Pretty(), err)
				}

				log.Debugf("Received new IPNS record from %s. Expiration %s", message.From().Pretty(), expiration)
			}
		}
	}()
	return nil
}
