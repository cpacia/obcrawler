package repo

import (
	"gorm.io/gorm"
	"time"
)

// Peer is the database model holding information about the peer
// and its IPNS record.
type Peer struct {
	PeerID         string `gorm:"primary_key"`
	FirstSeen      time.Time
	LastSeen       time.Time `gorm:"index"`
	LastCrawled    time.Time `gorm:"index"`
	LastPinned     time.Time `gorm:"index"`
	IPNSExpiration time.Time `gorm:"index"`
	IPNSRecord     []byte
	Banned         bool `gorm:"index"`
}

// CIDRecord is a database model that maps a CID to a peer ID.
type CIDRecord struct {
	gorm.Model
	CID    string `gorm:"index"`
	PeerID string `gorm:"index"`
}
