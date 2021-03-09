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

func SaveCoins(list []*CoinData) error {
	return nil
}

func UpdateCoinsWithOTAKey() error {
	return nil
}

func GetCoins(OTASecret string) ([]CoinData, error) {
	return nil, nil
}

func SaveUsedKeyimage(list []*KeyImageData) error {
	return nil
}

func CheckKeyimagesUsed(list []string) ([]bool, error) {
	return nil, nil
}
