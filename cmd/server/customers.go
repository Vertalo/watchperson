// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/base/log"
	"github.com/moov-io/watchman/internal/database"
	"github.com/moov-io/watchman/pkg/ofac"
)

// Customer is an individual on one or more SDN list(s)
type Customer struct {
	ID string `json:"id"`
	// Federal Data
	SDN       *ofac.SDN                 `json:"sdn"`
	Addresses []*ofac.Address           `json:"addresses"`
	Alts      []*ofac.AlternateIdentity `json:"alts"`
	Comments  []*ofac.SDNComments       `json:"comments"`
	// Metadata
	Status *CustomerStatus `json:"status"`
	Match  float64         `json:"match,omitempty"`
}

// CustomerBlockStatus can be either CustomerUnsafe or CustomerException
type CustomerBlockStatus string

const (
	// CustomerUnsafe customers have been manually marked to block all transactions with
	CustomerUnsafe CustomerBlockStatus = "unsafe"
	// CustomerException customers have been manually marked to allow transactions with
	CustomerException CustomerBlockStatus = "exception"
)

// CustomerStatus represents a userID's manual override of an SDN
type CustomerStatus struct {
	UserID    string              `json:"userID"`
	Note      string              `json:"note"`
	Status    CustomerBlockStatus `json:"block"`
	CreatedAt time.Time           `json:"createdAt"`
}

type sqliteCustomerRepository struct {
	db     *sql.DB
	logger log.Logger
}

func (r *sqliteCustomerRepository) close() error {
	return r.db.Close()
}

func (r *sqliteCustomerRepository) getCustomerStatus(customerID string) (*CustomerStatus, error) {
	if customerID == "" {
		return nil, errors.New("getCustomerStatus: no Customer.ID")
	}
	query := `select user_id, note, status, created_at from customer_status where customer_id = ? and deleted_at is null order by created_at desc limit 1;`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	row := stmt.QueryRow(customerID)

	var status CustomerStatus
	err = row.Scan(&status.UserID, &status.Note, &status.Status, &status.CreatedAt)
	if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
		return nil, fmt.Errorf("getCustomerStatus: %v", err)
	}
	if status.UserID == "" {
		return nil, nil // not found
	}
	return &status, nil
}

func (r *sqliteCustomerRepository) upsertCustomerStatus(customerID string, status *CustomerStatus) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("upsertCustomerStatus: begin: %v", err)
	}

	query := `insert into customer_status (customer_id, user_id, note, status, created_at) values (?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("upsertCustomerStatus: prepare error=%v rollback=%v", err, tx.Rollback())
	}
	_, err = stmt.Exec(customerID, status.UserID, status.Note, status.Status, status.CreatedAt)
	defer stmt.Close()
	if err == nil {
		return tx.Commit()
	}
	if database.UniqueViolation(err) {
		query = `update customer_status set note = ?, status = ? where customer_id = ? and user_id = ?`
		stmt, err = tx.Prepare(query)
		if err != nil {
			return fmt.Errorf("upsertCustomerStatus: inner prepare error=%v rollback=%v", err, tx.Rollback())
		}
		_, err := stmt.Exec(status.Note, status.Status, customerID, status.UserID)
		defer stmt.Close()
		if err != nil {
			return fmt.Errorf("upsertCustomerStatus: unique error=%v rollback=%v", err, tx.Rollback())
		}
	}
	return tx.Commit()
}