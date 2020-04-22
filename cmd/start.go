package cmd

import (
	"github.com/cpacia/obcrawler/crawler"
	"github.com/cpacia/obcrawler/repo"
	"github.com/op/go-logging"
	"os"
	"os/signal"
)

var log = logging.MustGetLogger("CMD")

// Start is the main entry point for the crawler. The options to this
// command are the same as the crawler config options.
type Start struct {
	repo.Config
}

// Execute starts the crawler.
func (x *Start) Execute(args []string) error {
	cfg, err := repo.LoadConfig()
	if err != nil {
		return err
	}

	crawler, err := crawler.NewCrawler(cfg)
	if err != nil {
		return err
	}

	if err := crawler.Start(); err != nil {
		return err
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	for sig := range c {
		if sig == os.Kill {
			log.Info("obcrawler killed")
			os.Exit(1)
		}

		if err := crawler.Start(); err != nil {
			log.Error(err)
			os.Exit(1)
		}
		log.Info("obcrawler stopping...")
		os.Exit(1)
	}
	return nil
}
