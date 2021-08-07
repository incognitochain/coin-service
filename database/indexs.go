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
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(50000)*shared.DB_OPERATION_TIMEOUT)
	coinMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "otasecret", Value: bsonx.Int32(1)}, {Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "coinidx", Value: bsonx.Int32(1)}},
		},
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "realtokenid", Value: bsonx.Int32(1)}, {Key: "otasecret", Value: bsonx.Int32(1)}, {Key: "coinidx", Value: bsonx.Int32(1)}},
		},
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "realtokenid", Value: bsonx.Int32(1)}, {Key: "otasecret", Value: bsonx.Int32(1)}},
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
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(1000)*shared.DB_OPERATION_TIMEOUT)
	txMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "keyimages", Value: bsonx.Int32(1)}},
		},
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

// func DBCreateTradeIndex() error {
// 	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
// 	model := []mongo.IndexModel{
// 		{
// 			Keys: bsonx.Doc{{Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "respondtx", Value: bsonx.Int32(1)}, {Key: "status", Value: bsonx.Int32(1)}},
// 		},
// 	}
// 	indexName, err := mgm.Coll(&shared.TradeData{}).Indexes().CreateMany(ctx, model)
// 	if err != nil {
// 		return err
// 	}
// 	log.Println("indexName", indexName)
// 	return nil
// }

func DBCreateShieldIndex() error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	model := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "respondtx", Value: bsonx.Int32(1)}, {Key: "height", Value: bsonx.Int32(-1)}},
		},
	}
	indexName, err := mgm.Coll(&shared.ShieldData{}).Indexes().CreateMany(ctx, model)
	if err != nil {
		return err
	}
	log.Println("indexName", indexName)
	return nil
}

func DBCreatePDEXIndex() error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	ctrbModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "contributor", Value: bsonx.Int32(1)}, {Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "respondblock", Value: bsonx.Int32(-1)}},
		},
	}
	_, err := mgm.Coll(&shared.ContributionData{}).Indexes().CreateMany(ctx, ctrbModel)
	if err != nil {
		return err
	}
	ctx, _ = context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	wdCtrbModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "contributor", Value: bsonx.Int32(1)}, {Key: "status", Value: bsonx.Int32(1)}, {Key: "respondtime", Value: bsonx.Int32(-1)}},
		},
	}
	_, err = mgm.Coll(&shared.WithdrawContributionData{}).Indexes().CreateMany(ctx, wdCtrbModel)
	if err != nil {
		return err
	}

	ctx, _ = context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	wdFeeCtrbModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "contributor", Value: bsonx.Int32(1)}, {Key: "status", Value: bsonx.Int32(1)}, {Key: "respondtime", Value: bsonx.Int32(-1)}},
		},
	}
	_, err = mgm.Coll(&shared.WithdrawContributionFeeData{}).Indexes().CreateMany(ctx, wdFeeCtrbModel)
	if err != nil {
		return err
	}

	ctx, _ = context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	tradeOrderModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "status", Value: bsonx.Int32(1)}, {Key: "pairid", Value: bsonx.Int32(1)}, {Key: "locktime", Value: bsonx.Int32(-1)}},
		},
		{
			Keys: bsonx.Doc{{Key: "status", Value: bsonx.Int32(1)}, {Key: "poolid", Value: bsonx.Int32(1)}, {Key: "locktime", Value: bsonx.Int32(-1)}},
		},
		{
			Keys: bsonx.Doc{{Key: "requesttx", Value: bsonx.Int32(1)}},
		},
	}
	_, err = mgm.Coll(&shared.TradeOrderData{}).Indexes().CreateMany(ctx, tradeOrderModel)
	if err != nil {
		return err
	}

	ctx, _ = context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	poolPairModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "pairid", Value: bsonx.Int32(1)}, {Key: "poolid", Value: bsonx.Int32(1)}},
		},
	}
	_, err = mgm.Coll(&shared.PoolPairData{}).Indexes().CreateMany(ctx, poolPairModel)
	if err != nil {
		return err
	}

	return nil
}
