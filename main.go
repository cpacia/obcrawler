package main

import (
	"context"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
	"github.com/op/go-logging"
	"gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"
	"strings"
	"fmt"
)

func init() {
	multiaddr.Protocols = append(multiaddr.Protocols, multiaddr.Protocol{477, 0, "ws", multiaddr.CodeToVarint(477), false, nil})
}

type Start struct {
	GatewayAddr    string `short:"g" long:"gatewayaddr" description:"the address of the openbazaar-go gateway" default:"127.0.0.1:4002"`
	APIServerAddr  string `short:"a" long:"apiaddr" description:"the address to bind the API server to" default:":8080"`
	DataDir        string `short:"d" long:"datadir" description:"directory to use for the persistent cache"`
	Hostname       string `short:"h" long:"hostname" description:"hostname for the chart server" default:"localhost"`
	CrawlDelay     int    `short:"c" long:"crawldelay" description:"time between crawls in seconds" default:"22"`
	NodeDelay      int    `short:"n" long:"nodedelay" description:"how long to wait before crawling a node again in seconds" default:"5400"`
	MaxConcurrency int    `short:"m" long:"maxconcurrency" description:"maximum number of nodes to crawl at one time" default:"20"`
	CrawlListings  bool   `short:"l" long:"crawllistings" description:"crawl each node's listings"`
	LogStats       bool   `short:"s" long:"logstats" description:"log stats to the database"`
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
	datadir, err := getRepoPath()
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		datadir = x.DataDir
	}
	os.Mkdir(datadir, os.ModePerm)
	db, err := NewBoltDatastore(datadir)
	if err != nil {
		return err
	}

	config := CrawlerConfig{
		client:         NewOBClient(x.GatewayAddr),
		crawlDelay:     time.Second * time.Duration(x.CrawlDelay),
		nodeDelay:      time.Second * time.Duration(x.NodeDelay),
		maxConcurrency: x.MaxConcurrency,
		crawlListings:  x.CrawlListings,
		db:             db,
	}
	crawler := NewCrawler(config)
	log.Notice("Starting crawler")
	go crawler.runCrawler(ctx)

	if x.LogStats {
		sl := NewStatsLogger(db, crawler.GetNodes)
		go sl.run()
	}

	p := strings.Split(x.APIServerAddr, ":")

	server := NewAPIServer(x.APIServerAddr, crawler, fmt.Sprintf("%s:%s", x.Hostname, p[1]))
	server.serve()
	return nil
}

func getRepoPath() (string, error) {
	// Set default base path and directory name
	path := "~"
	directoryName := "obcrawler"

	// Override OS-specific names
	switch runtime.GOOS {
	case "linux":
		directoryName = ".obcrawler"
	case "darwin":
		path = "~/Library/Application Support"
	}

	// Join the path and directory name, then expand the home path
	fullPath, err := homedir.Expand(filepath.Join(path, directoryName))
	if err != nil {
		return "", err
	}

	// Return the shortest lexical representation of the path
	return filepath.Clean(fullPath), nil
}