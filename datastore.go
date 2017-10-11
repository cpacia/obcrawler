package main

import (
	"crypto/rand"
	"encoding/json"
	"github.com/boltdb/bolt"
	"path"
	"sync"
)

// Datastore is use to persist nodes between sessions
type Datastore interface {
	PutNode(node Node) error
	GetAllNodes() ([]Node, error)
	PutStat(t StatType, stat Snapshot) error
	GetStats(t StatType) ([]Snapshot, error)
}

type BoltDatastore struct {
	db *bolt.DB
	l  sync.RWMutex
}

func NewBoltDatastore(filePath string) (*BoltDatastore, error) {
	filePath = path.Join(filePath, "datastore.bin")
	db, err := bolt.Open(filePath, 0644, &bolt.Options{InitialMmapSize: 5000000})
	if err != nil {
		return nil, err
	}
	db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists([]byte("nodes"))
		return err
	})
	db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists([]byte("stats_all"))
		return err
	})
	db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists([]byte("stats_clearnet"))
		return err
	})
	db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists([]byte("stats_toronly"))
		return err
	})
	db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists([]byte("stats_dualstack"))
		return err
	})
	return &BoltDatastore{db, sync.RWMutex{}}, nil
}

func (b *BoltDatastore) PutNode(node Node) error {
	b.l.Lock()
	defer b.l.Unlock()
	return b.db.Update(func(btx *bolt.Tx) error {
		nds := btx.Bucket([]byte("nodes"))
		ser, err := SerializeNode(node)
		if err != nil {
			return err
		}
		err = nds.Put([]byte(node.PeerInfo.ID.Pretty()), ser)
		if err != nil {
			return err
		}
		return nil
	})
}

func (b *BoltDatastore) GetAllNodes() ([]Node, error) {
	var nodes []Node
	b.l.RLock()
	defer b.l.RUnlock()
	err := b.db.View(func(btx *bolt.Tx) error {
		nds := btx.Bucket([]byte("nodes"))
		err := nds.ForEach(func(k, v []byte) error {
			node, err := DeserializeNode(v)
			if err != nil {
				return err
			}
			nodes = append(nodes, node)
			return nil
		})
		return err
	})
	return nodes, err
}

func SerializeNode(node Node) ([]byte, error) {
	return json.Marshal(&node)
}

func DeserializeNode(ser []byte) (Node, error) {
	node := new(Node)
	err := json.Unmarshal(ser, node)
	return *node, err
}

func (b *BoltDatastore) PutStat(t StatType, stat Snapshot) error {
	b.l.Lock()
	defer b.l.Unlock()

	var typeStr string
	switch t {
	case StatAll:
		typeStr = "stats_all"
	case StatClearnet:
		typeStr = "stats_clearnet"
	case StatTorOnly:
		typeStr = "stats_toronly"
	case StatDualStack:
		typeStr = "stats_dualstack"
	}

	return b.db.Update(func(btx *bolt.Tx) error {
		sn := btx.Bucket([]byte(typeStr))
		ser, err := SerializeSnapShot(stat)
		if err != nil {
			return err
		}
		key := make([]byte, 32)
		rand.Read(key)
		err = sn.Put(key, ser)
		if err != nil {
			return err
		}
		return nil
	})
}

func (b *BoltDatastore) GetStats(t StatType) ([]Snapshot, error) {
	var stats []Snapshot
	b.l.RLock()
	defer b.l.RUnlock()

	var typeStr string
	switch t {
	case StatAll:
		typeStr = "stats_all"
	case StatClearnet:
		typeStr = "stats_clearnet"
	case StatTorOnly:
		typeStr = "stats_toronly"
	case StatDualStack:
		typeStr = "stats_dualstack"
	}

	err := b.db.View(func(btx *bolt.Tx) error {
		sns := btx.Bucket([]byte(typeStr))
		err := sns.ForEach(func(k, v []byte) error {
			stat, err := DeserializeSnapShot(v)
			if err != nil {
				return err
			}
			stats = append(stats, stat)
			return nil
		})
		return err
	})
	return stats, err
}

func SerializeSnapShot(s Snapshot) ([]byte, error) {
	return json.Marshal(&s)
}

func DeserializeSnapShot(ser []byte) (Snapshot, error) {
	s := new(Snapshot)
	err := json.Unmarshal(ser, s)
	return *s, err
}
