// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/moov-io/base/log"
	"github.com/moov-io/watchman/internal/database"
	"go4.org/syncutil"
)

var (
	flagLogFormat = flag.String("log.format", "", "Format for log lines (Options: json, plain")
	flagMaxProcs  = flag.Int("max-procs", runtime.NumCPU(), "Maximum number of CPUs used for search and endpoints")
	flagWorkers   = flag.Int("workers", 1024, "Maximum number of goroutines used for search")

	flagInputFile = flag.String("file", "./data/input.tsv", "Input file to parse")
	flagDelimiter = flag.String("delimiter", "\t", "Delimiter for input file")
	flagThreshold = flag.Float64("threshold", .90, "Threshold for similarity")

	dataRefreshInterval = 1 * time.Hour
)

type FileRow struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// parseFile only handles files that can be split on a delimiter for now
// A future iteration of this function should be as an implementation of
// a FileParser interface
func parseFile(path string, delimiter string) ([]FileRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	regex := regexp.MustCompile(delimiter)

	var rows []FileRow

	for scanner.Scan() {
		words := regex.Split(scanner.Text(), 3)
		row := FileRow{Id: words[0], Name: words[2], Email: words[1]}

		rows = append(rows, row)
	}

	return rows, nil
}

func main() {
	flag.Parse()

	runtime.GOMAXPROCS(*flagMaxProcs)

	var logger log.Logger
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		*flagLogFormat = v
	}
	if strings.ToLower(*flagLogFormat) == "json" {
		logger = log.NewJSONLogger()
	} else {
		logger = log.NewDefaultLogger()
	}

	// Channel for errors
	errs := make(chan error)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("signal: %v", <-c)
	}()

	// Setup database connection
	db, err := database.New(logger, os.Getenv("DATABASE_TYPE"))
	if err != nil {
		logger.Logf("database problem: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.LogError(err)
		}
	}()

	// Setup download repository
	downloadRepo := &sqliteDownloadRepository{db, logger}
	defer downloadRepo.close()

	var pipeline *pipeliner
	if debug, err := strconv.ParseBool(os.Getenv("DEBUG_NAME_PIPELINE")); debug && err == nil {
		pipeline = newPipeliner(logger)
	} else {
		pipeline = newPipeliner(log.NewNopLogger())
	}
	searcher := newSearcher(logger, pipeline, *flagWorkers)

	// Initial download of data
	if stats, err := searcher.refreshData(os.Getenv("INITIAL_DATA_DIRECTORY")); err != nil {
		logger.LogErrorf("ERROR: failed to download/parse initial data: %v", err)
		os.Exit(1)
	} else {
		if err := downloadRepo.recordStats(stats); err != nil {
			logger.LogErrorf("ERROR: failed to record download stats: %v", err)
			os.Exit(1)
		}
		logger.Info().With(log.Fields{
			"SDNs":        log.Int(stats.SDNs),
			"AltNames":    log.Int(stats.Alts),
			"Addresses":   log.Int(stats.Addresses),
			"SSI":         log.Int(stats.SectoralSanctions),
			"DPL":         log.Int(stats.DeniedPersons),
			"BISEntities": log.Int(stats.BISEntities),
		}).Logf("data refreshed %v ago", time.Since(stats.RefreshedAt))
	}

	// Setup company / customer repositories
	custRepo := &sqliteCustomerRepository{db, logger}
	defer custRepo.close()

	// Setup periodic download and re-search
	updates := make(chan *DownloadStats)
	dataRefreshInterval = getDataRefreshInterval(logger, os.Getenv("DATA_REFRESH_INTERVAL"))
	go searcher.periodicDataRefresh(dataRefreshInterval, downloadRepo, updates)

	// Parse input file
	rows, err := parseFile(*flagInputFile, *flagDelimiter)

	if err != nil {
		logger.Fatal()
	}

	rows = rows[1:]

	var wg sync.WaitGroup
	var arr []searchResponse

	wg.Add(len(rows))

	workers := syncutil.NewGate(*flagWorkers)

	for _, row := range rows {
		workers.Start()
		go func(row FileRow) {
			defer workers.Done()
			defer wg.Done()

			resp := buildFullSearchResponse(searcher, 1, *flagThreshold, row.Name, row.Email)
			arr = append(arr, *resp)
		}(row)
	}

	wg.Wait()

	data, err := json.MarshalIndent(arr, "", "\t")

	if err != nil {
		logger.LogErrorf("ERROR: failed to marshal search results: %v", err)
	}

	if err := os.WriteFile("./data/output.json", data, 0644); err != nil {
		logger.LogErrorf("ERROR: failed to write search results: %v", err)
	}
}

// getDataRefreshInterval returns a time.Duration for how often OFAC should refresh data
func getDataRefreshInterval(logger log.Logger, env string) time.Duration {
	if env != "" {
		if strings.EqualFold(env, "off") {
			return 0 * time.Second
		}
		if dur, _ := time.ParseDuration(env); dur > 0 {
			logger.Logf("Setting data refresh interval to %v", dur)
			return dur
		}
	}
	logger.Logf("Setting data refresh interval to %v (default)", dataRefreshInterval)
	return dataRefreshInterval
}
