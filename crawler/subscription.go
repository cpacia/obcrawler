package crawler

type Subscription struct {
	Close func() error
	Out   <-chan interface{}
}
