// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"sync"
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

// searchGather performs an inmem search with *searcher and mutates *searchResponse by setting a specific field
type searchGather func(searcher *searcher, limit int, minMatch float64, name string, resp *searchResponse)

var (
	gatherings = []searchGather{
		// OFAC SDN Search
		func(s *searcher, limit int, minMatch float64, name string, resp *searchResponse) {
			sdns := s.FindSDNsByRemarksID(limit, name)
			if len(sdns) == 0 {
				resp.SDNs = s.TopSDNs(limit, minMatch, name)

				if len(resp.SDNs) > 0 {
					resp.Match = &resp.SDNs[0].match
				}
			}
		},
	}
)

func buildFullSearchResponse(searcher *searcher, limit int, minMatch float64, name string, email string) *searchResponse {
	resp := searchResponse{
		Email:       email,
		FullName:    name,
		RefreshedAt: searcher.lastRefreshedAt,
	}
	var wg sync.WaitGroup
	wg.Add(len(gatherings))
	for i := range gatherings {
		go func(i int) {
			gatherings[i](searcher, limit, minMatch, name, &resp)
			wg.Done()
		}(i)
	}
	wg.Wait()

	resp.Hash = resp.HashResponse()

	return &resp
}
