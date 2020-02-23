package repo

import "time"

type Peer struct {
	PeerID         string `gorm:"primary_key"`
	FirstSeen      time.Time
	LastSeen       time.Time
	LastCrawled    time.Time
	IPNSExpiration time.Time
	IPNSRecord     []byte
	Banned         bool
}
