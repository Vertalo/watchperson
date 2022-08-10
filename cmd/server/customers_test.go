// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"testing"
	"time"

	"github.com/moov-io/base"
	"github.com/moov-io/base/log"
	"github.com/moov-io/watchman/internal/database"
	"github.com/moov-io/watchman/pkg/ofac"
)

var (
	// customerSearcher is a searcher instance with one individual entity contained. It's designed to be used
	// in tests that expect an individual SDN.
	customerSearcher *searcher
)

func init() {
	customerSearcher = newSearcher(log.NewNopLogger(), noLogPipeliner, 1)
	customerSearcher.SDNs = precomputeSDNs([]*ofac.SDN{
		{
			EntityID: "306",
			SDNName:  "BANCO NACIONAL DE CUBA",
			SDNType:  "individual",
			Programs: []string{"CUBA"},
			Title:    "",
			Remarks:  "a.k.a. 'BNC'.",
		},
	}, nil, noLogPipeliner)
	customerSearcher.Addresses = precomputeAddresses([]*ofac.Address{
		{
			EntityID:                    "306",
			AddressID:                   "201",
			Address:                     "Dai-Ichi Bldg. 6th Floor, 10-2 Nihombashi, 2-chome, Chuo-ku",
			CityStateProvincePostalCode: "Tokyo 103",
			Country:                     "Japan",
		},
	})
	customerSearcher.Alts = precomputeAlts([]*ofac.AlternateIdentity{
		{
			EntityID:      "306",
			AlternateID:   "220",
			AlternateType: "aka",
			AlternateName: "NATIONAL BANK OF CUBA",
		},
	}, noLogPipeliner)
}

func TestCustomerRepository(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, repo *sqliteCustomerRepository) {
		customerID, userID := base.ID(), base.ID()

		status, err := repo.getCustomerStatus(customerID)
		if err != nil {
			t.Fatal(err)
		}
		if status != nil {
			t.Fatal("should give nil CustomerStatus")
		}

		// block customer
		status = &CustomerStatus{UserID: userID, Status: CustomerUnsafe, CreatedAt: time.Now()}
		if err := repo.upsertCustomerStatus(customerID, status); err != nil {
			t.Errorf("addCustomerBlock: shouldn't error, but got %v", err)
		}

		// verify
		status, err = repo.getCustomerStatus(customerID)
		if err != nil {
			t.Error(err)
		}
		if status == nil {
			t.Fatal("empty CustomerStatus")
		}
		if status.UserID == "" || string(status.Status) == "" {
			t.Errorf("invalid CustomerStatus: %#v", status)
		}
		if status.Status != CustomerUnsafe {
			t.Errorf("status.Status=%v", status.Status)
		}

		// unblock
		status = &CustomerStatus{UserID: userID, Status: CustomerException, CreatedAt: time.Now()}
		if err := repo.upsertCustomerStatus(customerID, status); err != nil {
			t.Errorf("addCustomerBlock: shouldn't error, but got %v", err)
		}

		status, err = repo.getCustomerStatus(customerID)
		if err != nil {
			t.Error(err)
		}
		if status == nil {
			t.Fatal("empty CustomerStatus")
		}
		if status.UserID == "" || string(status.Status) == "" {
			t.Errorf("invalid CustomerStatus: %#v", status)
		}
		if status.Status != CustomerException {
			t.Errorf("status.Status=%v", status.Status)
		}

		// edgae case
		status, err = repo.getCustomerStatus("")
		if status != nil {
			t.Error("empty customerID shouldn return nil status")
		}
		if err == nil {
			t.Error("but an error should be returned")
		}
	}

	// SQLite tests
	sqliteDB := database.CreateTestSqliteDB(t)
	defer sqliteDB.Close()
	check(t, &sqliteCustomerRepository{sqliteDB.DB, log.NewNopLogger()})
}
