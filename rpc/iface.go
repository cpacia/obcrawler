package rpc

import (
	peer "github.com/libp2p/go-libp2p-peer"
)

type Crawler interface {
	Subscribe() (*Subscription, error)
	CrawlNode(pid peer.ID) error
	BanNode(pid peer.ID) error
	UnbanNode(pid peer.ID) error
}
