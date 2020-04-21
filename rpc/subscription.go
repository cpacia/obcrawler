package rpc

import "time"

type Subscription struct {
	Close func() error
	Out   chan *Object
}

type Object struct {
	Data           interface{}
	ExpirationDate time.Time
}
