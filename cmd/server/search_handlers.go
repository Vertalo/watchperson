// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
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

func buildFullSearchResponse(searcher *searcher, limit int, minMatch float64, name string, email string) *searchResponse {
	resp := searchResponse{
		Email:       email,
		FullName:    name,
		RefreshedAt: searcher.lastRefreshedAt,
	}

	sdns := searcher.TopSDNs(limit, minMatch, name)

	if len(sdns) > 0 {
		resp.Match = &sdns[0].match
	}

	resp.SDNs = sdns
	resp.Hash = resp.HashResponse()

	return &resp
}
