package main

import (
	"gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	"time"
)

type Node struct {
	peerInfo    peerstore.PeerInfo // holds peerID and list of multiaddrs
	lastConnect time.Time          // last time we sucessfully connected to this client
	lastTry     time.Time          // last time we tried to connect to this client
	crawlStart  time.Time          // time when we started the last crawl
	statusStr   string             // string with last error or OK details
	userAgent   string             // remote client user agent
	nFails      uint32             // number of times we have failed to crawl this client
	status      uint32             // rg,cg,wg,ng
	rating      uint32             // if it reaches 100 then we mark them statusNG
	crawlActive bool               // are we currently crawling this client
}

