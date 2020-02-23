package main

import (
	"github.com/cpacia/obcrawler/cmd"
	"github.com/jessevdk/go-flags"
	"log"
	"os"
)

func main() {
	parser := flags.NewParser(nil, flags.Default)

	_, err := parser.AddCommand("start",
		"start the crawler",
		"The start command starts the crawler",
		&cmd.Start{})
	if err != nil {
		log.Fatal(err)
	}

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}
