package database

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

func DBCreateCoinV1Index() error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	coinMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "coinpubkey", Value: bsonx.Int32(1)}, {Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "coinidx", Value: bsonx.Int32(1)}},
		},
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "coinidx", Value: bsonx.Int32(1)}},
		},
		{
			Keys:    bsonx.Doc{{Key: "coin", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := mgm.Coll(&shared.CoinDataV1{}).Indexes().CreateMany(ctx, coinMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}

	ctx, _ = context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	keyInfoMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "pubkey", Value: bsonx.Int32(1)}},
		},
	}
	_, err = mgm.Coll(&shared.KeyInfoData{}).Indexes().CreateMany(ctx, keyInfoMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}
	log.Printf("success index coins in %v", time.Since(startTime))
	return nil
}

func DBCreateCoinV2Index() error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	coinMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "otasecret", Value: bsonx.Int32(1)}, {Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "coinidx", Value: bsonx.Int32(1)}},
		},
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "realtokenid", Value: bsonx.Int32(1)}, {Key: "otasecret", Value: bsonx.Int32(1)}, {Key: "coinidx", Value: bsonx.Int32(1)}},
		},
		{
			Keys:    bsonx.Doc{{Key: "coinpubkey", Value: bsonx.Int32(1)}, {Key: "coin", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := mgm.Coll(&shared.CoinData{}).Indexes().CreateMany(ctx, coinMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}

	ctx, _ = context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	otaMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "indexerid", Value: bsonx.Int32(1)}},
		},
		{
			Keys:    bsonx.Doc{{Key: "otakey", Value: bsonx.Int32(1)}, {Key: "pubkey", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err = mgm.Coll(&shared.SubmittedOTAKeyData{}).Indexes().CreateMany(ctx, otaMdl)
	if err != nil {
		log.Printf("failed to index otakey in %v", time.Since(startTime))
		return err
	}

	ctx, _ = context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	keyInfoMdl := []mongo.IndexModel{
		{
			Keys:    bsonx.Doc{{Key: "otakey", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err = mgm.Coll(&shared.KeyInfoDataV2{}).Indexes().CreateMany(ctx, keyInfoMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}
	log.Printf("success index coins in %v", time.Since(startTime))

	return nil
}

func DBCreateKeyimageIndex() error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	imageMdl := []mongo.IndexModel{
		{
			Keys:    bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "keyimage", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "txhash", Value: bsonx.Int32(1)}},
		},
	}
	indexName, err := mgm.Coll(&shared.KeyImageData{}).Indexes().CreateMany(ctx, imageMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}
	log.Println("indexName", indexName)
	log.Printf("success index keyimages in %v", time.Since(startTime))
	return nil
}

func DBCreateTxIndex() error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	txMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "locktime", Value: bsonx.Int32(-1)}},
		},
		{
			Keys:    bsonx.Doc{{Key: "txhash", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "keyimages", Value: bsonx.Int32(1)}, {Key: "locktime", Value: bsonx.Int32(-1)}},
		},
	}
	indexName, err := mgm.Coll(&shared.TxData{}).Indexes().CreateMany(ctx, txMdl)
	if err != nil {
		return err
	}
	log.Println("indexName", indexName)
	return nil
}

func DBCreateTxPendingIndex() error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	txMdl := []mongo.IndexModel{
		{
			Keys:    bsonx.Doc{{Key: "txhash", Value: bsonx.Int32(1)}, {Key: "shardid", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	indexName, err := mgm.Coll(&shared.CoinPendingData{}).Indexes().CreateMany(ctx, txMdl)
	if err != nil {
		return err
	}
	log.Println("indexName", indexName)
	return nil
}
