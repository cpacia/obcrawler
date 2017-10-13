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
	db             Datastore
}

type Crawler struct {
	theList        map[string]*Node
	client         *OBClient
	crawlListings  bool
	crawlDelay     time.Duration
	nodeDelay      time.Duration
	maxConcurrency int
	lock           sync.RWMutex
	db             Datastore
}

func NewCrawler(config CrawlerConfig) *Crawler {
	return &Crawler{
		client:         config.client,
		crawlListings:  config.crawlListings,
		crawlDelay:     config.crawlDelay,
		nodeDelay:      config.nodeDelay,
		theList:        make(map[string]*Node),
		maxConcurrency: config.maxConcurrency,
		lock:           sync.RWMutex{},
		db:             config.db,
	}
}

func (c *Crawler) runCrawler(ctx context.Context) {
	// Load peers from database
	nodes, err := c.db.GetAllNodes()
	if err != nil {
		log.Error(err)
		return
	}
	log.Noticef("Loaded %d nodes from database", len(nodes))
	for _, node := range nodes {
		n := node
		c.theList[node.PeerInfo.ID.Pretty()] = &n
	}

	// Fetch current peers
	log.Notice("Fetching connected peers")
	currentPeers, err := c.client.Peers()
	if err != nil {
		log.Error(err)
		return
	}
	go c.logStats()
	var wg sync.WaitGroup
	for _, p := range currentPeers {
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			c.lock.RLock()
			if _, ok := c.theList[p.Pretty()]; ok {
				c.lock.RUnlock()
				return
			}
			c.lock.RUnlock()

			pi, err := c.client.PeerInfo(p)
			if err != nil {
				log.Warningf("Couldn't find addrs for peer %s\n", p.Pretty())
			}
			ua, err := c.client.UserAgent(p)
			if err != nil {
				log.Warningf("Couldn't find user agent for peer %s\n", p.Pretty())
			}
			n := Node{
				PeerInfo:    pi,
				UserAgent:   ua,
				LastConnect: time.Now(),
			}
			c.lock.Lock()
			defer c.lock.Unlock()
			if _, ok := c.theList[pi.ID.Pretty()]; !ok {
				log.Debugf("Found new node: %s\n", pi.ID.Pretty())
				c.theList[pi.ID.Pretty()] = &n
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
		if nd.CrawlActive {
			continue
		}

		// Do we already have enough started
		if started > c.maxConcurrency {
			continue
		}

		// Don't crawl a node to quickly
		if nd.LastTry.Add(c.nodeDelay).After(time.Now()) {
			continue
		}

		// All looks good so start a go routine to crawl the remote node
		nd.CrawlActive = true
		nd.CrawlStart = time.Now()

		go c.crawlNode(nd)
		started++
	}
	if started == 0 {
		log.Info("Idling...")
	}
}

func (c *Crawler) crawlNode(nd *Node) {
	defer func() {
		nd.CrawlActive = false
		nd.LastTry = time.Now()
		if c.db != nil {
			c.db.PutNode(*nd)
		}
	}()
	log.Debugf("Crawling %s\n", nd.PeerInfo.ID.Pretty())
	if len(nd.PeerInfo.Addrs) == 0 {
		pi, err := c.client.PeerInfo(nd.PeerInfo.ID)
		if err != nil {
			log.Warningf("Couldn't find addrs for peer %s\n", nd.PeerInfo.ID.Pretty())
		}
		nd.PeerInfo = pi
	}
	if nd.UserAgent == "" || nd.LastConnect.Add(time.Hour*48).Before(time.Now()) {
		ua, err := c.client.UserAgent(nd.PeerInfo.ID)
		if err != nil {
			log.Warningf("Couldn't find user agent for peer %s\n", nd.PeerInfo.ID.Pretty())
		}
		nd.UserAgent = ua
	}
	online, err := c.client.Ping(nd.PeerInfo.ID)
	if err != nil {
		log.Errorf("Error pinging peer %s\n", nd.PeerInfo.ID.Pretty())
	}
	if online {
		nd.LastConnect = time.Now()
	}

	profile, err := c.client.Profile(nd.PeerInfo.ID)
	if err == nil && profile != nil && profile.Stats != nil {
		nd.Listings = int(profile.Stats.ListingCount)
		nd.Ratings = int(profile.Stats.RatingCount)
		nd.Vendor = profile.Vendor
	}


	closer, _ := c.client.ClosestPeers(nd.PeerInfo.ID)
	for _, p := range closer {
		if p.Pretty() == "" {
			continue
		}
		c.lock.Lock()
		_, ok := c.theList[p.Pretty()]
		if !ok {
			log.Debugf("Found new node: %s\n", p.Pretty())
			c.theList[p.Pretty()] = &Node{PeerInfo: ps.PeerInfo{ID: p}}
		}
		c.lock.Unlock()
	}
	if c.crawlListings {
		// TODO
	}
}

func (c *Crawler) logStats() {
	t := time.NewTicker(time.Minute)
	for range t.C {
		c.lock.RLock()
		total := len(c.theList)
		totalWithIP := 0
		totalTor := 0
		totalDualStack := 0
		totalClearnet := 0
		listings := 0
		ratings := 0
		for _, nd := range c.theList {
			ratings += nd.Ratings
			listings += nd.Listings
			if len(nd.PeerInfo.Addrs) > 0 {
				totalWithIP++
			} else {
				continue
			}
			nodeType := GetNodeType(nd.PeerInfo.Addrs)
			switch {
			case nodeType == TorOnly:
				totalTor++
			case nodeType == DualStack:
				totalDualStack++
			case nodeType == Clearnet:
				totalClearnet++
			}
		}
		log.Infof("Total PeerIDs: %d, Total with addrs: %d, Total Clearnet: %d, Total Tor: %d, Total Dualstack: %d, Total Listings: %d, Total Ratings: %d\n", total, totalWithIP, totalClearnet, totalTor, totalDualStack, listings, ratings)
		c.lock.RUnlock()
	}
}

func (c *Crawler) GetNodes() []Node {
	c.lock.RLock()
	defer c.lock.RUnlock()

	var nodes []Node
	for _, nd := range c.theList {
		nodes = append(nodes, *nd)
	}
	return nodes
}