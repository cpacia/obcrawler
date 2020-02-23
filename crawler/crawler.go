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

func (c *Crawler) CrawlNode(peer peer.ID) error {
	return nil
}

func (c *Crawler) BanNode(peer peer.ID) error {
	return nil
}

func (c *Crawler) UnBanNode(peer peer.ID) error {
	return nil
}

func (c *Crawler) BanData(cid cid.Cid) error {
	return nil
}

func (c *Crawler) UnBanData(cid cid.Cid) error {
	return nil
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
		ticker := time.NewTicker(c.crawlInterval)
		for {
			select {
			case <-ticker.C:
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
			case <-c.shutdown:
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
