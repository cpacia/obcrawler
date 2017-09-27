package main

import (
	"context"
	ps "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	"gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"sync"
	"time"
)

type CrawlerConfig struct {
	client         *OBClient
	crawlListings  bool
	crawlDelay     time.Duration
	nodeDelay      time.Duration
	maxConcurrency int
}

type Crawler struct {
	theList        map[string]Node
	client         *OBClient
	crawlListings  bool
	crawlDelay     time.Duration
	nodeDelay      time.Duration
	maxConcurrency int
	lock           sync.RWMutex
}

func NewCrawler(config CrawlerConfig) *Crawler {
	return &Crawler{
		client:         config.client,
		crawlListings:  config.crawlListings,
		crawlDelay:     config.crawlDelay,
		nodeDelay:      config.nodeDelay,
		theList:        make(map[string]Node),
		maxConcurrency: config.maxConcurrency,
		lock:           sync.RWMutex{},
	}
}

func (c *Crawler) runCrawler(ctx context.Context) {
	log.Notice("Fetching connected peers")
	currentPeers, err := c.client.Peers()
	if err != nil {
		log.Error(err)
		return
	}
	var wg sync.WaitGroup
	for _, p := range currentPeers {
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			pi, err := c.client.PeerInfo(p)
			if err != nil {
				log.Errorf("Couldn't find addrs for peer %s\n", p.Pretty())
				return
			}
			ua, err := c.client.UserAgent(p)
			if err != nil {
				log.Errorf("Couldn't find user agent for peer %s\n", p.Pretty())
				return
			}
			n := Node{
				peerInfo:    pi,
				userAgent:   ua,
				lastConnect: time.Now(),
			}
			c.lock.Lock()
			defer c.lock.Unlock()
			if _, ok := c.theList[pi.ID.Pretty()]; !ok {
				c.theList[pi.ID.Pretty()] = n
				log.Debugf("Found new node: %s\n", pi.ID.Pretty())
			}
		}(p)
	}
	wg.Wait()
	crawlChan := time.NewTicker(c.crawlDelay).C
mainLoop:
	for {
		select {
		case <-crawlChan:
			go c.crawlNodes()
		case <-ctx.Done():
			break mainLoop
		}
	}
}

func (c *Crawler) crawlNodes() {
	tcount := uint32(len(c.theList))
	if tcount == 0 {
		log.Warningf("crwalNodes fail: no node available")
		return
	}

	started := 0
	c.lock.RLock()
	defer c.lock.RUnlock()

	for _, nd := range c.theList {
		if nd.crawlActive == true {
			continue
		}

		// Do we already have enough started
		if started > c.maxConcurrency {
			continue
		}

		// Don't crawl a node to quickly
		if nd.lastTry.Add(c.nodeDelay).After(time.Now()) {
			continue
		}

		// All looks good so start a go routine to crawl the remote node
		nd.crawlActive = true
		nd.crawlStart = time.Now()

		go c.crawlNode(nd)
		started++
	}
	if started == 0 {
		log.Info("Idling...")
	}
}

func (c *Crawler) crawlNode(nd Node) {
	defer func() {
		nd.lastTry = time.Now()
		c.lock.Lock()
		defer c.lock.Unlock()
		c.theList[nd.peerInfo.ID.Pretty()] = nd
	}()
	log.Debugf("Crawling %s\n", nd.peerInfo.ID.Pretty())
	if len(nd.peerInfo.Addrs) == 0 {
		pi, err := c.client.PeerInfo(nd.peerInfo.ID)
		if err != nil {
			log.Errorf("Couldn't find addrs for peer %s\n", nd.peerInfo.ID.Pretty())
			return
		}
		nd.peerInfo = pi
	}
	if nd.userAgent == "" || nd.lastConnect.Add(time.Hour*48).Before(time.Now()){
		ua, err := c.client.UserAgent(nd.peerInfo.ID)
		if err != nil {
			log.Errorf("Couldn't find user agent for peer %s\n", nd.peerInfo.ID.Pretty())
			return
		}
		nd.userAgent = ua
	}
	/*online, err := c.client.Ping(nd.peerInfo.ID)
	if err != nil {
		log.Errorf("Error pinging peer %s\n", nd.peerInfo.ID.Pretty())
		return
	}
	if online {
		nd.lastConnect = time.Now()
	}*/

	closer, err := c.client.ClosestPeers(nd.peerInfo.ID)
	if err != nil {
		log.Error("Error looking up closest peers")
		return
	}
	for _, p := range closer {
		c.lock.Lock()
		_, ok := c.theList[p.Pretty()]
		if !ok {
			log.Debugf("Found new node: %s\n", p.Pretty())
			c.theList[p.Pretty()] = Node{peerInfo: ps.PeerInfo{ID: p}}
		}
		c.lock.Unlock()
	}
	if c.crawlListings {
		// TODO
	}
}
