package main

// var dbSession *gocqlx.Session

// func connectDBCluster(nodes []string) error {
// 	cluster := gocql.NewCluster(nodes...)
// 	// Wrap session on creation, gocqlx session embeds gocql.Session pointer.
// 	session, err := gocqlx.WrapSession(cluster.CreateSession())
// 	if err != nil {
// 		return err
// 	}
// 	dbSession = &session
// 	return nil
// }
import (
	"context"
	"log"
	"time"

	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func connectDB() error {
	err := mgm.SetDefaultConfig(nil, "coins", options.Client().ApplyURI("mongodb://root:example@localhost:27017"))
	return err
}

func DBSaveCoins(list []CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	docs := []interface{}{}
	for _, coin := range list {
		docs = append(docs, coin)
	}
	_, err := mgm.Coll(&list[0]).InsertMany(ctx, docs)
	if err != nil {
		log.Printf("failed to insert %v coins in %v", len(list), time.Since(startTime))
		return err
	}
	log.Printf("inserted %v coins in %v", len(list), time.Since(startTime))
	return nil
}

func DBUpdateCoinsWithOTAKey() error {

	return nil
}

func DBGetCoins(OTASecret string) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"otasecret": bson.M{operator.Eq: OTASecret}}
	err := mgm.Coll(&CoinData{}).SimpleFind(&list, filter)
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBSaveUsedKeyimage(list []*KeyImageData) error {
	return nil
}

func DBCheckKeyimagesUsed(list []string) ([]bool, error) {
	return nil, nil
}
