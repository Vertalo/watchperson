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
	"syscall"
	"time"

	"github.com/moov-io/base/log"
	"github.com/moov-io/watchman/internal/database"
)

var (
	flagWorkers = flag.Int("workers", runtime.NumCPU()-1, "Maximum number of goroutines used for search")

	flagInputFile     = flag.String("input-file", "./data/input.tsv", "Input file to parse")
	flagOutputFile    = flag.String("output-file", "./data/output.json", "Output file to write")
	flagDelimiter     = flag.String("delimiter", "\t", "Delimiter for input file")
	flagThreshold     = flag.Float64("threshold", .95, "Threshold for similarity")
	flagSearchResults = flag.Int("search-results", 1000, "Number of search results to return at most")
	flagLimitFileRows = flag.Int("limit-file-rows", 0, "Limit the number of rows in the input file")
	flagDataDirectory = flag.String("data-directory", "", "Directory to download data to")
	flagSqliteFile    = flag.String("sqlite-db-path", "watchman.db", "Sqlite file to use")
)

type FileRow struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	flag.Parse()
	logger := log.NewDefaultLogger()

	if v := os.Getenv("THRESHOLD"); v != "" && !flagPassed("threshold") {
		threshold, err := strconv.ParseFloat(v, 64)
		if err != nil {
			logger.LogErrorf("invalid THRESHOLD: %v", err)
		}
		*flagThreshold = threshold
	}

	if v := os.Getenv("SQLITE_DB_PATH"); v != "" && !flagPassed("sqlite-db-path") {
		*flagSqliteFile = v
	}

	lastRefreshed := lastRefresh(*flagSqliteFile)
	if time.Since(lastRefreshed) > (time.Hour * 12) {
		logger.Logf("last refresh was %v ago, refreshing data", time.Since(lastRefreshed))
		if err := os.Remove(*flagSqliteFile); err != nil {
			logger.LogErrorf("error removing %s: %v", *flagSqliteFile, err)
		}
	}

	if v := os.Getenv("INITIAL_DATA_DIRECTORY"); v != "" && !flagPassed("data-directory") {
		*flagDataDirectory = v
	}

	// Channel for errors
	errs := make(chan error)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("signal: %v", <-c)
	}()

	// Setup database connection
	db, err := database.New(os.Getenv("DATABASE_TYPE"))
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
	if stats, err := searcher.refreshData(*flagDataDirectory); err != nil {
		logger.LogErrorf("ERROR: failed to download/parse initial data: %v", err)
		os.Exit(1)
	} else {
		logger.Info().With(log.Fields{
			"SDNs":      log.Int(stats.SDNs),
			"AltNames":  log.Int(stats.Alts),
			"Addresses": log.Int(stats.Addresses),
		}).Logf("data refreshed %v ago", time.Since(stats.RefreshedAt))
	}

	// Setup company / customer repositories
	custRepo := &sqliteCustomerRepository{db, logger}
	defer custRepo.close()

	// Parse input file
	rows, err := parseFile(*flagInputFile, *flagDelimiter)

	if err != nil {
		logger.Fatal()
	}

	// Remove input file
	if err = os.Remove(*flagInputFile); err != nil {
		logger.LogErrorf("ERROR: failed to remove input file: %v", err)
	}

	rows = rows[1:]

	if *flagLimitFileRows > 0 {
		rows = rows[:*flagLimitFileRows]
	}

	var arr []*searchResponse

	for _, row := range rows {
		resp := buildFullSearchResponse(searcher, *flagSearchResults, *flagThreshold, row.Name, row.Email)
		arr = append(arr, resp)
	}

	data, err := json.Marshal(arr)

	if err != nil {
		logger.LogErrorf("ERROR: failed to marshal search results: %v", err)
	}

	fmt.Printf("%s", data)

	if err := os.WriteFile(*flagOutputFile, data, 0644); err != nil {
		logger.LogErrorf("ERROR: failed to write output file: %v", err)
	}

	os.RemoveAll(*flagDataDirectory)
}

func flagPassed(name string) bool {
	found := false

	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})

	return found
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
