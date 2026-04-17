// Package runner provides the core scraping orchestration logic for google-maps-scraper.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/gosom/google-maps-scraper/gmaps"
	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/adapters/writers/csvwriter"
	"github.com/gosom/scrapemate/scrapematapp"
)

// Config holds the configuration for the scraper runner.
type Config struct {
	// Concurrency is the number of parallel browser tabs to use.
	Concurrency int
	// MaxDepth is the maximum depth to follow links (0 = seed only).
	MaxDepth int
	// InputFile is the path to a file containing search queries, one per line.
	InputFile string
	// ResultsFile is the path to write CSV results (empty = stdout).
	ResultsFile string
	// JSON enables JSON output instead of CSV.
	JSON bool
	// Language sets the language/locale for Google Maps requests.
	Language string
	// Debug enables verbose debug logging.
	Debug bool
	// ExitOnInactivity exits the scraper after N minutes of no new jobs.
	ExitOnInactivityMins int
}

// Runner orchestrates scraping jobs.
type Runner struct {
	cfg    Config
	logger *slog.Logger
}

// New creates a new Runner with the provided configuration.
func New(cfg Config) *Runner {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	return &Runner{cfg: cfg, logger: logger}
}

// Run starts the scraping process and blocks until completion or context cancellation.
func (r *Runner) Run(ctx context.Context) error {
	queries, err := r.loadQueries()
	if err != nil {
		return fmt.Errorf("loading queries: %w", err)
	}
	if len(queries) == 0 {
		return fmt.Errorf("no queries provided")
	}

	var resultsWriter io.Writer = os.Stdout
	if r.cfg.ResultsFile != "" {
		f, err := os.Create(r.cfg.ResultsFile)
		if err != nil {
			return fmt.Errorf("creating results file: %w", err)
		}
		defer f.Close()
		resultsWriter = f
	}

	var writer scrapemate.ResultWriter
	if r.cfg.JSON {
		writer = newJSONWriter(resultsWriter)
	} else {
		writer = csvwriter.NewCsvWriter(resultsWriter)
	}

	seeds := make([]scrapemate.IJob, 0, len(queries))
	for _, q := range queries {
		seeds = append(seeds, gmaps.NewGmapJob(q, r.cfg.Language, r.cfg.MaxDepth))
	}

	appCfg := scrapematapp.Config{
		Concurrency:  r.cfg.Concurrency,
		ResultWriter: writer,
		InitialJobs:  seeds,
		Logger:       r.logger,
	}

	r.logger.Info("starting scraper", "queries", len(queries), "concurrency", r.cfg.Concurrency)
	return scrapematapp.Run(ctx, appCfg)
}

// loadQueries reads search queries from InputFile or stdin.
func (r *Runner) loadQueries() ([]string, error) {
	var reader io.Reader
	if r.cfg.InputFile != "" {
		f, err := os.Open(r.cfg.InputFile)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		reader = f
	} else {
		reader = os.Stdin
	}
	return gmaps.ReadQueries(reader)
}

// jsonWriter writes scrape results as newline-delimited JSON.
type jsonWriter struct {
	mu  sync.Mutex
	out io.Writer
}

func newJSONWriter(w io.Writer) scrapemate.ResultWriter {
	return &jsonWriter{out: w}
}

func (j *jsonWriter) Run(ctx context.Context, in <-chan scrapemate.Result) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case res, ok := <-in:
			if !ok {
				return nil
			}
			b, err := json.Marshal(res.Data)
			if err != nil {
				continue
			}
			j.mu.Lock()
			_, _ = fmt.Fprintf(j.out, "%s\n", b)
			j.mu.Unlock()
		}
	}
}
