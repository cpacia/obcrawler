package repo

import "time"

type Peer struct {
	PeerID         string `gorm:"primary_key"`
	FirstSeen      time.Time
	LastSeen       time.Time
	LastCrawled    time.Time `gorm:"index"`
	IPNSExpiration time.Time `gorm:"index"`
	IPNSRecord     []byte
	Banned         bool `gorm:"index"`
}

type CIDRecord struct {
	CID    string `gorm:"primary_key"`
	PeerID string `gorm:"index"`
}
