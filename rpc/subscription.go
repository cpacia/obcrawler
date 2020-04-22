package rpc

import "time"

// Subscription represents a subscription to the data
// streamed by the crawler.
type Subscription struct {
	Close func() error
	Out   chan *Object
}

// Object is streamed to the subscription's out chan.
type Object struct {
	Data           interface{}
	ExpirationDate time.Time
}
