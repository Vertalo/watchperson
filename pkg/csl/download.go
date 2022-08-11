// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package csl

import (
	"fmt"
	"net/url"
	"os"

	"github.com/moov-io/base/log"
	"github.com/moov-io/watchman/pkg/download"
)

var (
	cslDownloadTemplate = func() string {
		if w := os.Getenv("CSL_DOWNLOAD_TEMPLATE"); w != "" {
			return w
		}
		return "https://api.trade.gov/static/consolidated_screening_list/%s"
	}()
)

func Download(logger log.Logger, initialDir string) (string, error) {
	dl := download.New(logger, download.HTTPClient)

	cslURL, err := buildDownloadURL(cslDownloadTemplate)
	if err != nil {
		return "", err
	}

	cslNameAndSource := make(map[string]string)
	cslNameAndSource["csl.csv"] = cslURL

	file, err := dl.GetFiles(initialDir, cslNameAndSource)
	if len(file) == 0 || err != nil {
		return "", fmt.Errorf("csl download: %v", err)
	}
	return file[0], nil
}

func buildDownloadURL(urlStr string) (string, error) {
	cslURL, err := url.Parse(fmt.Sprintf(urlStr, "consolidated.csv"))
	if err != nil {
		return "", err
	}
	return cslURL.String(), nil
}
