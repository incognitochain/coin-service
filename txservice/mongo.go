package main

import (
	"context"
	"log"
	"time"

	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectDB() error {
	err := mgm.SetDefaultConfig(nil, "main", options.Client().ApplyURI(MONGODB))
	if err != nil {
		return err
	}
	_, cd, _, _ := mgm.DefaultConfigs()
	err = cd.Ping(context.Background(), nil)
	if err != nil {
		return err
	}
	log.Println("Database Connected!")
	return nil
}

func getAllFailedTx() ([]TxData, error) {
	var result []TxData
	filter := bson.M{"status": bson.M{operator.In: []string{txStatusFailed}}}
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Second)
	err := mgm.Coll(&TxData{}).SimpleFindWithCtx(ctx, &result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func saveTx(tx TxData) error {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	var doc interface{}
	tx.Creating()
	doc = tx
	_, err := mgm.Coll(&TxData{}).InsertOne(ctx, doc)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func updateTxStatus(txhash, status string) error {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	filter := bson.M{"txhash": bson.M{operator.Eq: txhash}}
	var doc interface{}
	update := bson.M{
		"$set": bson.M{"status": status},
	}
	doc = update
	_, err := mgm.Coll(&TxData{}).UpdateOne(ctx, filter, doc)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func deleteTx(txHash string) error {
	return nil
}
