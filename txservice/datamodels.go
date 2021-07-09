package main

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

type TxData struct {
	mgm.DefaultModel `bson:",inline"`
	Raw              string `json:"raw" bson:"raw"`
	TxHash           string `json:"txhash" bson:"txhash"`
	Status           string `json:"status" bson:"status"`
	Error            string `json:"error" bson:"error"`
}

func NewTxData(txhash, raw, status, err string) *TxData {
	return &TxData{TxHash: txhash, Raw: raw, Status: status, Error: err}
}

func (model *TxData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TxData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

func DBCreateTxIndex() error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	imageMdl := []mongo.IndexModel{
		{
			Keys:    bsonx.Doc{{Key: "txhash", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	indexName, err := mgm.Coll(&TxData{}).Indexes().CreateMany(ctx, imageMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}
	log.Println("indexName", indexName)
	log.Printf("success index keyimages in %v", time.Since(startTime))
	return nil
}
