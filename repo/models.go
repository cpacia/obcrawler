package repo

import "time"

// Peer is the database model holding information about the peer
// and its IPNS record.
type Peer struct {
	PeerID         string `gorm:"primary_key"`
	FirstSeen      time.Time
	LastSeen       time.Time
	LastCrawled    time.Time `gorm:"index"`
	IPNSExpiration time.Time `gorm:"index"`
	IPNSRecord     []byte
	Banned         bool `gorm:"index"`
}

// CIDRecord is a database model that maps a CID to a peer ID.
type CIDRecord struct {
	CID    string `gorm:"primary_key"`
	PeerID string `gorm:"index"`
}
