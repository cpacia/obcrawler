package main

import (
	"gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	"time"
)

type Node struct {
	PeerInfo    peerstore.PeerInfo `json:"peerInfo"`    // holds peerID and list of multiaddrs
	LastConnect time.Time          `json:"lastConnect"` // last time we sucessfully connected to this client
	LastTry     time.Time          `json:"lastTry"`     // last time we tried to connect to this client
	CrawlStart  time.Time          `json:"crawlStart"`  // time when we started the last crawl
	UserAgent   string             `json:"userAgent"`   // remote client user agent
	CrawlActive bool               `json:"crawlActive"` // are we currently crawling this client
	Listings    int                `json:"listings"`    // number of listings node has
	Ratings     int                `json:"ratings"`     // number of ratings
	Vendor      bool               `json:"vendor"`      // is node a vendor
}
