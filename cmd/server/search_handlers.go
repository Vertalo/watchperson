// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

type searchResponse struct {
	// OFAC
	SDNs     []*SDN   `json:"SDNs"`
	Email    string   `json:"email"`
	FullName string   `json:"fullName"`
	Match    *float64 `json:"match"`
	Hash     string   `json:"hash"`

	// Metadata
	RefreshedAt time.Time `json:"refreshedAt"`
}

func (s searchResponse) HashResponse() string {
	buffer := new(bytes.Buffer)
	hasher := sha1.New()

	s.RefreshedAt = time.Time{}

	json.NewEncoder(buffer).Encode(s)
	hasher.Write(buffer.Bytes())
	return hex.EncodeToString(hasher.Sum(nil))
}

func findTopMatchValue(sdns []*SDN) float64 {
	var topMatch float64

	for _, sdn := range sdns {
		if sdn.match > topMatch {
			topMatch = sdn.match
		}
	}

	return topMatch
}

func buildFullSearchResponse(searcher *searcher, limit int, minMatch float64, name string, email string) *searchResponse {
	resp := searchResponse{
		Email:       email,
		FullName:    name,
		RefreshedAt: searcher.lastRefreshedAt,
	}

	sdns := searcher.TopSDNs(limit, minMatch, name)
	topMatch := findTopMatchValue(sdns)

	// Remove all values lower than topMatch
	for i, sdn := range sdns {
		if sdn.match >= topMatch {
			resp.SDNs = append(resp.SDNs, sdns[i+1:]...)
		}
	}

	sort.Slice(resp.SDNs, func(i, j int) bool {
		x, _ := strconv.Atoi(resp.SDNs[i].id)
		y, _ := strconv.Atoi(resp.SDNs[j].id)
		return x < y
	})

	resp.Match = &topMatch
	resp.SDNs = sdns
	resp.Hash = resp.HashResponse()

	return &resp
}
