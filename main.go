package main

import (
	"gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	"time"
	"github.com/op/go-logging"
	"os"
	"context"
	"os/signal"
	"github.com/jessevdk/go-flags"
)

func init() {
	multiaddr.Protocols = append(multiaddr.Protocols, multiaddr.Protocol{477, 0, "ws", multiaddr.CodeToVarint(477), false, nil},)
}

type Start struct {
	GatewayAddr    string   `short:"g" long:"gatewayaddr" description:"the address of the openbazaar-go gateway" default:"127.0.0.1:4002"`
	APIServerAddr  string   `short:"a" long:"apiaddr" description:"the address to bind the API server to" default:":8080"`
	CrawlDelay     int      `short:"c" long:"crawldelay" description:"time between crawls in seconds" default:"22"`
	NodeDelay      int      `short:"n" long:"nodedelay" description:"how long to wait before crawling a node again in seconds" default:"5400"`
	MaxConcurrency int      `short:"m" long:"maxconcurrency" description:"maximum number of nodes to crawl at one time" default:"20"`
	CrawlListings  bool     `short:"l" long:"crawllistings" description:"crawl each node's listings"`
}

var stdoutLogFormat = logging.MustStringFormatter(
	`%{color:reset}%{color}%{time:15:04:05.000} [%{shortfunc}] [%{level}] %{message}`,
)

var start Start

var log = logging.MustGetLogger("main")

var parser = flags.NewParser(nil, flags.Default)
var ctx context.Context

func main() {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			cancel()
			log.Info("Shutting down")
			os.Exit(1)
		}
	}()
	parser.AddCommand("start",
		"start the crawler",
		"Runs the crawler with the given options",
		&start)
	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}

func (x *Start) Execute(args []string) error {
	backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
	backendStdoutFormatter := logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
	logging.SetBackend(backendStdoutFormatter)

	config := CrawlerConfig{
		client: NewOBClient(x.GatewayAddr),
		crawlDelay: time.Second*time.Duration(x.CrawlDelay),
		nodeDelay: time.Second*time.Duration(x.NodeDelay),
		maxConcurrency: x.MaxConcurrency,
		crawlListings: x.CrawlListings,
	}
	crawler := NewCrawler(config)
	log.Notice("Starting crawler")
	go crawler.runCrawler(ctx)

	server := NewAPIServer(x.APIServerAddr, crawler)
	server.serve()
	return nil
}
