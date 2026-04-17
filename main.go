package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gosom/google-maps-scraper/scraper"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var cfg scraper.Config

	flag.StringVar(&cfg.InputFile, "input", "", "path to input file with search queries (one per line)")
	flag.StringVar(&cfg.OutputFile, "output", "output.csv", "path to output CSV file")
	flag.IntVar(&cfg.Concurrency, "concurrency", 2, "number of concurrent browser tabs")
	flag.IntVar(&cfg.MaxDepth, "depth", 10, "max results per query")
	flag.BoolVar(&cfg.Debug, "debug", false, "enable debug logging")
	flag.BoolVar(&cfg.Headless, "headless", true, "run browser in headless mode")
	flag.StringVar(&cfg.Lang, "lang", "en", "language code for Google Maps (e.g. en, de, fr)")
	flag.BoolVar(&cfg.GeoCoordinates, "geo", false, "extract geo coordinates")

	printVersion := flag.Bool("version", false, "print version and exit")

	flag.Parse()

	if *printVersion {
		fmt.Printf("google-maps-scraper version=%s commit=%s date=%s\n", version, commit, date)
		return nil
	}

	if cfg.InputFile == "" {
		// fall back to positional args
		args := flag.Args()
		if len(args) == 0 {
			return fmt.Errorf("no input provided: use -input flag or pass queries as arguments")
		}
		cfg.Queries = args
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s, err := scraper.New(cfg)
	if err != nil {
		return fmt.Errorf("initialising scraper: %w", err)
	}
	defer s.Close()

	return s.Run(ctx)
}
