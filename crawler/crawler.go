package crawler

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/cpacia/obcrawler/repo"
	"github.com/cpacia/obcrawler/rpc"
	"github.com/cpacia/openbazaar3.0/core"
	obrepo "github.com/cpacia/openbazaar3.0/repo"
	"github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-cid"
	core2 "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/corerepo"
	ipnspb "github.com/ipfs/go-ipns/pb"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	inet "github.com/libp2p/go-libp2p-core/network"
	peer "github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
	"github.com/op/go-logging"
	"gorm.io/gorm"
	mrand "math/rand"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var log = logging.MustGetLogger("CRWLR")

// Crawler is an OpenBazaar network crawler which seeks to
// scrape all new listings and profiles.
type Crawler struct {
	nodes         []*core.OpenBazaarNode
	numPubsub     uint
	numWorkers    uint
	ipnsQuorum    uint
	workChan      chan *job
	cacheData     bool
	pinFiles      bool
	pinRecords    bool
	subs          map[uint64]*rpc.Subscription
	subMtx        sync.RWMutex
	db            *repo.Database
	ctx           context.Context
	cancel        context.CancelFunc
	crawlInterval time.Duration
	grpcServer    *rpc.GrpcServer
	resolver      *resolver
	shutdown      chan struct{}
}

// NewCrawler returns a new crawler with the given config options.
func NewCrawler(cfg *repo.Config) (*Crawler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	crawler := &Crawler{
		ctx:           ctx,
		cancel:        cancel,
		workChan:      make(chan *job),
		subs:          make(map[uint64]*rpc.Subscription),
		subMtx:        sync.RWMutex{},
		cacheData:     !cfg.DisableDataCaching,
		pinFiles:      !cfg.DisableFilePinning,
		pinRecords:    !cfg.DisableIPNSPinning,
		numPubsub:     cfg.PubsubNodes,
		numWorkers:    cfg.NumWorkers,
		ipnsQuorum:    cfg.IPNSQuorum,
		crawlInterval: cfg.CrawlInterval,
		shutdown:      make(chan struct{}),
	}
	for i := 0; i < int(cfg.NumNodes); i++ {
		nodeConfig := &obrepo.Config{
			DataDir:           path.Join(cfg.DataDir, "nodes", strconv.Itoa(i)),
			LogDir:            path.Join(cfg.DataDir, "nodes", strconv.Itoa(i), "logs"),
			DisableNATPortMap: cfg.DisableNATPortMap,
			UserAgentComment:  cfg.UserAgentComment,
			EnabledWallets:    nil,
			SNFServerPeers:    nil,
			IPFSOnly:          true,
			LogLevel:          cfg.LogLevel,
			IPNSQuorum:        cfg.IPNSQuorum,
			Testnet:           cfg.Testnet,
			SwarmAddrs: []string{
				fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", strconv.Itoa(4009+i)),
			},
		}
		if len(cfg.BoostrapAddrs) > 0 {
			nodeConfig.BoostrapAddrs = cfg.BoostrapAddrs
		}

		n, err := core.NewNode(ctx, nodeConfig)
		if err != nil {
			return nil, err
		}
		crawler.nodes = append(crawler.nodes, n)
	}

	opts := []repo.Option{
		repo.User(cfg.DBUser),
		repo.Password(cfg.DBPass),
		repo.Dialect(cfg.DBDialect),
	}
	if cfg.DBHost != "" {
		if !strings.Contains(cfg.DBHost, ":") {
			return nil, errors.New("invalid host")
		}
		s := strings.Split(cfg.DBHost, ":")

		port, err := strconv.Atoi(s[1])
		if err != nil {
			return nil, err
		}

		opts = append(opts, repo.Host(s[0]), repo.Port(uint(port)))
	}

	db, err := repo.NewDatabase(cfg.DataDir, opts...)
	if err != nil {
		return nil, err
	}

	crawler.db = db

	if err := repo.CheckAndSetUlimit(); err != nil {
		return nil, err
	}

	if len(cfg.GrpcListeners) > 0 {
		netAddrs, err := parseListeners(cfg.GrpcListeners)
		if err != nil {
			return nil, err
		}
		crawler.grpcServer, err = newGrpcServer(netAddrs, crawler, cfg)
		if err != nil {
			return nil, err
		}
	}

	if len(cfg.ResolverListeners) > 0 {
		netAddrs, err := parseListeners(cfg.ResolverListeners)
		if err != nil {
			return nil, err
		}
		crawler.resolver = newResolver(netAddrs, db, cfg)
	}

	return crawler, nil
}

// CrawlNode is a method that can be used to manually trigger a
// crawl of a node. If the node is banned an error will be returned.
func (c *Crawler) CrawlNode(pid peer.ID) error {
	err := c.db.View(func(db *gorm.DB) error {
		var peer repo.Peer
		err := db.Where("peer_id=?", pid.Pretty()).First(&peer).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if peer.Banned {
			return errors.New("peer is banned")
		}
		return nil
	})
	if err != nil {
		return err
	}
	go func() {
		c.workChan <- &job{
			Peer:           pid,
			FetchNewRecord: true,
			PinRecord:      c.pinRecords,
		}
	}()
	return nil
}

// BanNode bans the provided node and unpins any content of that node
// that the crawler is seeding. The content will then be delete at the
// next gc interval.
//
// Once a node is banned it will no longer be crawled going forward.
func (c *Crawler) BanNode(pid peer.ID) error {
	var cidsRecs []repo.CIDRecord
	err := c.db.Update(func(db *gorm.DB) error {
		var peer repo.Peer
		err := db.Where("peer_id=?", pid.Pretty()).First(&peer).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		peer.PeerID = pid.Pretty()
		peer.Banned = true
		if err := db.Save(&peer).Error; err != nil {
			return err
		}
		return db.Where("peer_id=?", pid.Pretty()).Find(&cidsRecs).Error
	})
	if err != nil {
		return err
	}
	for _, rec := range cidsRecs {
		id, err := cid.Decode(rec.CID)
		if err != nil {
			continue
		}

		if err := c.unpinCID(id); err != nil {
			log.Error("Error unpinning data for banned node %s: %s", pid.String(), err)
		}
	}

	return nil
}

// UnbanNode marks the node as unbanned in the database and make it
// eligible to once again be crawled.
func (c *Crawler) UnbanNode(pid peer.ID) error {
	return c.db.Update(func(db *gorm.DB) error {
		var peer repo.Peer
		err := db.Where("peer_id=?", pid.Pretty()).First(&peer).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		peer.PeerID = pid.String()
		peer.Banned = false
		return db.Save(&peer).Error
	})
}

// Subscribe returns a subscription with a channel over which new profiles
// and listings will be pushed when they are crawled.
func (c *Crawler) Subscribe() (*rpc.Subscription, error) {
	i := mrand.Uint64()
	sub := &rpc.Subscription{
		Out: make(chan *rpc.Object),
		Close: func() error {
			c.subMtx.Lock()
			defer c.subMtx.Unlock()

			s, ok := c.subs[i]
			if ok {
				close(s.Out)
				delete(c.subs, i)
			}
			return nil
		},
	}

	c.subMtx.Lock()
	defer c.subMtx.Unlock()

	c.subs[i] = sub

	return sub, nil
}

// Start will start the crawler and related processes.
func (c *Crawler) Start() error {
	for _, n := range c.nodes {
		n.Start()
		c.listenPeers(n.IPFSNode())
	}
	go func() {
		crawlTicker := time.NewTicker(c.crawlInterval)
		gcTicker := time.NewTicker(time.Hour * 24)
		oldNodeTicker := time.NewTicker(time.Minute)
		unPinTicker := time.NewTicker(time.Hour)
		for {
			select {
			case <-crawlTicker.C:
				r := mrand.Intn(len(c.nodes))
				_, pub, err := crypto.GenerateEd25519Key(rand.Reader)
				if err != nil {
					log.Errorf("Error generating random key: %s", err)
					continue
				}
				randPeer, err := peer.IDFromPublicKey(pub)
				if err != nil {
					log.Errorf("Error generating peer ID: %s", err)
					continue
				}
				log.Debugf("Node %d starting network crawl", r)
				_, err = c.nodes[r].IPFSNode().Routing.FindPeer(context.Background(), randPeer)
				if err != routing.ErrNotFound {
					log.Errorf("Error crawling for more peers %s", err)
				}
			case <-gcTicker.C:
				for _, n := range c.nodes {
					corerepo.GarbageCollectAsync(n.IPFSNode(), c.ctx)
				}
			case <-oldNodeTicker.C:
				var peers []repo.Peer
				err := c.db.View(func(db *gorm.DB) error {
					return db.Where("banned=?", false).
						Where("ip_ns_expiration>?", time.Now()).
						Where("last_crawled<?", time.Now().Add(-time.Hour*24*7)).
						Where("last_seen>?", time.Now().Add(-time.Hour*24*90)).
						Order("last_crawled asc").
						Limit(10).
						Find(&peers).Error
				})
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					log.Errorf("Error crawling loading old peers %s", err)
					continue
				}
				go func() {
					for _, p := range peers {
						pid, err := peer.Decode(p.PeerID)
						if err != nil {
							log.Errorf("Error decoding peerID in old node loop: %s", err)
							continue
						}
						rec := new(ipnspb.IpnsEntry)
						if err := proto.Unmarshal(p.IPNSRecord, rec); err != nil {
							log.Errorf("Error unmarshalling IPNS record for peer %s: %s", p.PeerID, err)
							continue
						}
						c.workChan <- &job{
							Peer:           pid,
							IPNSRecord:     rec,
							FetchNewRecord: true,
							PinRecord:      c.pinRecords,
							Expiration:     p.IPNSExpiration,
						}
					}
				}()
			case <-unPinTicker.C:
				var peers []repo.Peer
				err := c.db.View(func(db *gorm.DB) error {
					return db.Where("ip_ns_expiration>?", time.Now()).
						Where("last_seen>?", time.Now().Add(-time.Hour*24*90)).
						Order("last_crawled asc").
						Limit(10).
						Find(&peers).Error
				})
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					log.Errorf("Error loading old peers %s", err)
					continue
				}
				go func() {
					for _, p := range peers {
						var cidsRecs []repo.CIDRecord
						err := c.db.View(func(db *gorm.DB) error {
							return db.Where("peer_id=?", p.PeerID).Find(&cidsRecs).Error
						})
						if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
							log.Errorf("Error loading cid for dead peer %s", err)
							continue
						}
						for _, rec := range cidsRecs {
							id, err := cid.Decode(rec.CID)
							if err != nil {
								continue
							}

							if err := c.unpinCID(id); err != nil {
								log.Error("Error unpinning data for dead node %s: %s", p.PeerID, err)
							}
						}
					}
				}()
			case <-c.shutdown:
				crawlTicker.Stop()
				gcTicker.Stop()
				oldNodeTicker.Stop()
				return
			}
		}
	}()
	for i := 0; i < int(c.numWorkers); i++ {
		go c.worker()
	}
	return c.listenPubsub()
}

// Stop shuts down the crawler.
func (c *Crawler) Stop() error {
	close(c.shutdown)
	c.cancel()
	for _, n := range c.nodes {
		if err := n.Stop(false); err != nil {
			return err
		}
	}
	return nil
}

func (c *Crawler) notifySubscribers(obj *rpc.Object) {
	c.subMtx.RLock()
	for _, sub := range c.subs {
		sub.Out <- obj
	}
	c.subMtx.RUnlock()
}

func (c *Crawler) unpinCID(id cid.Cid) error {
	for _, n := range c.nodes {
		capi, err := coreapi.NewCoreAPI(n.IPFSNode())
		if err != nil {
			return err
		}
		capi.Pin().Rm(c.ctx, ipath.IpfsPath(id))
	}
	return nil
}

func (c *Crawler) listenPeers(n *core2.IpfsNode) {
	updatePeer := func(_ inet.Network, conn inet.Conn) {
		err := c.db.Update(func(db *gorm.DB) error {
			var peer repo.Peer
			err := db.Where("peer_id=?", conn.RemotePeer().Pretty()).First(&peer).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				peer.FirstSeen = time.Now()
				peer.PeerID = conn.RemotePeer().Pretty()
				log.Infof("Detected new peer: %s", conn.RemotePeer().Pretty())
			}
			peer.LastSeen = time.Now()
			return db.Save(&peer).Error
		})
		if err != nil {
			log.Errorf("Error updating database on peer connection: %s", err)
		}
	}

	notifier := &inet.NotifyBundle{
		ConnectedF:    updatePeer,
		DisconnectedF: updatePeer,
	}
	n.PeerHost.Network().Notify(notifier)
}
