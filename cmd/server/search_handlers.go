// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"sync"
	"time"

	"github.com/moov-io/watchman/pkg/csl"
)

type searchResponse struct {
	// OFAC
	SDNs      []*SDN    `json:"SDNs"`
	AltNames  []Alt     `json:"altNames"`
	Addresses []Address `json:"addresses"`
	Email     string    `json:"email"`
	FullName  string    `json:"fullName"`

	// BIS
	DeniedPersons []DP `json:"deniedPersons"`

	// Consolidated Screening List
	BISEntities       []*Result[csl.EL]  `json:"bisEntities"`
	MilitaryEndUsers  []*Result[csl.MEU] `json:"militaryEndUsers"`
	SectoralSanctions []*Result[csl.SSI] `json:"sectoralSanctions"`

	// Metadata
	RefreshedAt time.Time `json:"refreshedAt"`
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
			}
		},
		// OFAC SDN Alt Names
		func(s *searcher, limit int, minMatch float64, name string, resp *searchResponse) {
			resp.AltNames = s.TopAltNames(limit, minMatch, name)
		},
		// OFAC Addresses
		func(s *searcher, limit int, minMatch float64, name string, resp *searchResponse) {
			resp.Addresses = s.TopAddresses(limit, minMatch, name)
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
	return &resp
}
