package rpc

import (
	peer "github.com/libp2p/go-libp2p-core/peer"
)

// Crawler is an interface to the Crawler package used to
// avoid circular imports.
type Crawler interface {
	Subscribe() (*Subscription, error)
	CrawlNode(pid peer.ID) error
	BanNode(pid peer.ID) error
	UnbanNode(pid peer.ID) error
}
