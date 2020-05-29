package main

import (
	"github.com/cpacia/obcrawler/crawler"
	"github.com/cpacia/obcrawler/repo"
	"github.com/op/go-logging"
	"os"
	"os/signal"
)

var log = logging.MustGetLogger("MAIN")

func main() {
	cfg, err := repo.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	crawler, err := crawler.NewCrawler(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if err := crawler.Start(); err != nil {
		log.Fatal(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	for sig := range c {
		if sig == os.Kill {
			log.Info("obcrawler killed")
			os.Exit(1)
		}

		if err := crawler.Stop(); err != nil {
			log.Error(err)
			os.Exit(1)
		}
		log.Info("obcrawler stopping...")
		os.Exit(0)
	}
}
