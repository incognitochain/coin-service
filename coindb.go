package main

import (
	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx/v2"
)

var dbSession *gocqlx.Session

func connectDBCluster(nodes []string) error {
	cluster := gocql.NewCluster(nodes...)
	// Wrap session on creation, gocqlx session embeds gocql.Session pointer.
	session, err := gocqlx.WrapSession(cluster.CreateSession())
	if err != nil {
		return err
	}
	dbSession = &session
	return nil
}

func DBSaveCoins(list []*CoinData) error {
	return nil
}

func DBUpdateCoinsWithOTAKey() error {
	return nil
}

func DBGetCoins(OTASecret string) ([]CoinData, error) {
	return nil, nil
}

func DBSaveUsedKeyimage(list []*KeyImageData) error {
	return nil
}

func DBCheckKeyimagesUsed(list []string) ([]bool, error) {
	return nil, nil
}
