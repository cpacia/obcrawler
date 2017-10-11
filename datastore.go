package main

import (
	"encoding/json"
	"github.com/boltdb/bolt"
	"path"
	"sync"
)

// Datastore is use to persist nodes between sessions
type Datastore interface {
	Put(node Node) error

	GetAllNodes() ([]Node, error)
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
	return &BoltDatastore{db, sync.RWMutex{}}, nil
}

func (b *BoltDatastore) Put(node Node) error {
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
