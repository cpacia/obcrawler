package crawler

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/cpacia/obcrawler/repo"
	"github.com/cpacia/openbazaar3.0/core"
	obrepo "github.com/cpacia/openbazaar3.0/repo"
	"github.com/ipfs/go-cid"
	core2 "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/corerepo"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/jinzhu/gorm"
	crypto "github.com/libp2p/go-libp2p-crypto"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	routing "github.com/libp2p/go-libp2p-routing"
	"github.com/op/go-logging"
	mrand "math/rand"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var log = logging.MustGetLogger("CRWLR")

type Crawler struct {
	nodes         []*core.OpenBazaarNode
	numPubsub     uint
	numWorkers    uint
	ipnsQuorum    uint
	workChan      chan *Job
	cacheData     bool
	pinFiles      bool
	subs          map[uint64]*Subscription
	subMtx        sync.RWMutex
	db            *repo.Database
	ctx           context.Context
	cancel        context.CancelFunc
	crawlInterval time.Duration
	shutdown      chan struct{}
}

func NewCrawler(cfg *repo.Config) (*Crawler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	crawler := &Crawler{
		ctx:           ctx,
		cancel:        cancel,
		workChan:      make(chan *Job),
		subs:          make(map[uint64]*Subscription),
		subMtx:        sync.RWMutex{},
		cacheData:     !cfg.DisableDataCaching,
		pinFiles:      !cfg.DisableFilePinning,
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

	return crawler, nil
}

func (c *Crawler) CrawlNode(pid peer.ID) error {
	err := c.db.View(func(db *gorm.DB) error {
		var peer repo.Peer
		err := db.Where("peer_id=?", pid.Pretty()).First(&peer).Error
		if err != nil && !gorm.IsRecordNotFoundError(err) {
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
		c.workChan <- &Job{
			Peer: pid,
		}
	}()
	return nil
}

func (c *Crawler) BanNode(pid peer.ID) error {
	var cidsRecs []repo.CIDRecord
	err := c.db.Update(func(db *gorm.DB) error {
		var peer repo.Peer
		err := db.Where("peer_id=?", pid.Pretty()).First(&peer).Error
		if err != nil && !gorm.IsRecordNotFoundError(err) {
			return err
		}
		peer.PeerID = pid.String()
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

func (c *Crawler) UnBanNode(pid peer.ID) error {
	return c.db.Update(func(db *gorm.DB) error {
		var peer repo.Peer
		err := db.Where("peer_id=?", pid.Pretty()).First(&peer).Error
		if err != nil && !gorm.IsRecordNotFoundError(err) {
			return err
		}
		peer.PeerID = pid.String()
		peer.Banned = false
		return db.Save(&peer).Error
	})
}

func (c *Crawler) Subscribe() (*Subscription, error) {
	i := mrand.Uint64()
	sub := &Subscription{
		Out: make(chan *Object),
		Close: func() error {
			c.subMtx.Lock()
			defer c.subMtx.Unlock()

			delete(c.subs, i)
			return nil
		},
	}

	c.subMtx.Lock()
	defer c.subMtx.Unlock()

	c.subs[i] = sub

	return sub, nil
}

func (c *Crawler) Start() error {
	for _, n := range c.nodes {
		n.Start()
		c.listenPeers(n.IPFSNode())
	}
	go func() {
		crawlTicker := time.NewTicker(c.crawlInterval)
		gcTicker := time.NewTicker(time.Hour * 24)
		oldNodeTicker := time.NewTicker(time.Minute)
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
						Where("ip_ns_expiration<?", time.Now()).
						Where("last_crawled<?", time.Now().Add(-time.Hour*24*7)).
						Order("last_crawled asc").
						Limit(10).
						Find(&peers).Error
				})
				if err != nil && !gorm.IsRecordNotFoundError(err) {
					log.Errorf("Error crawling loading old peers %s", err)
					continue
				}
				go func() {
					for _, p := range peers {
						pid, err := peer.IDB58Decode(p.PeerID)
						if err != nil {
							log.Errorf("Error decoding peerID in old node loop: %s", err)
							continue
						}
						c.workChan <- &Job{
							Peer: pid,
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
			if err != nil && !gorm.IsRecordNotFoundError(err) {
				return err
			} else if gorm.IsRecordNotFoundError(err) {
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
