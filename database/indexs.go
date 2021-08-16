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
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "tokenid", Value: bsonx.Int32(1)}},
		},
	}
	_, err := mgm.Coll(&shared.CoinDataV1{}).Indexes().CreateMany(context.Background(), coinMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}

	keyInfoMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "pubkey", Value: bsonx.Int32(1)}},
		},
	}
	_, err = mgm.Coll(&shared.KeyInfoData{}).Indexes().CreateMany(context.Background(), keyInfoMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}
	log.Printf("success index coins in %v", time.Since(startTime))
	return nil
}

func DBCreateCoinV2Index() error {
	startTime := time.Now()
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
	_, err := mgm.Coll(&shared.CoinData{}).Indexes().CreateMany(context.Background(), coinMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}

	otaMdl := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "indexerid", Value: bsonx.Int32(1)}},
		},
		{
			Keys:    bsonx.Doc{{Key: "otakey", Value: bsonx.Int32(1)}, {Key: "pubkey", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err = mgm.Coll(&shared.SubmittedOTAKeyData{}).Indexes().CreateMany(context.Background(), otaMdl)
	if err != nil {
		log.Printf("failed to index otakey in %v", time.Since(startTime))
		return err
	}

	keyInfoMdl := []mongo.IndexModel{
		{
			Keys:    bsonx.Doc{{Key: "otakey", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err = mgm.Coll(&shared.KeyInfoDataV2{}).Indexes().CreateMany(context.Background(), keyInfoMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}
	log.Printf("success index coins in %v", time.Since(startTime))

	return nil
}

func DBCreateKeyimageIndex() error {
	startTime := time.Now()
	imageMdl := []mongo.IndexModel{
		{
			Keys:    bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "keyimage", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "txhash", Value: bsonx.Int32(1)}},
		},
	}
	indexName, err := mgm.Coll(&shared.KeyImageData{}).Indexes().CreateMany(context.Background(), imageMdl)
	if err != nil {
		log.Printf("failed to index coins in %v", time.Since(startTime))
		return err
	}
	log.Println("indexName", indexName)
	log.Printf("success index keyimages in %v", time.Since(startTime))
	return nil
}

func DBCreateTxIndex() error {
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
		{
			Keys: bsonx.Doc{{Key: "shardid", Value: bsonx.Int32(1)}, {Key: "locktime", Value: bsonx.Int32(-1)}},
		},
		{
			Keys: bsonx.Doc{{Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "pubkeyreceivers", Value: bsonx.Int32(1)}, {Key: "txversion", Value: bsonx.Int32(1)}, {Key: "locktime", Value: bsonx.Int32(-1)}},
		},
		{
			Keys: bsonx.Doc{{Key: "realtokenid", Value: bsonx.Int32(1)}, {Key: "pubkeyreceivers", Value: bsonx.Int32(1)}, {Key: "txversion", Value: bsonx.Int32(1)}, {Key: "locktime", Value: bsonx.Int32(-1)}},
		},
	}
	indexName, err := mgm.Coll(&shared.TxData{}).Indexes().CreateMany(context.Background(), txMdl)
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
	model := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "respondtx", Value: bsonx.Int32(1)}, {Key: "height", Value: bsonx.Int32(-1)}},
		},
	}
	indexName, err := mgm.Coll(&shared.ShieldData{}).Indexes().CreateMany(context.Background(), model)
	if err != nil {
		return err
	}
	log.Println("indexName", indexName)
	return nil
}

func DBCreatePDEXIndex() error {
	ctrbModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "contributor", Value: bsonx.Int32(1)}, {Key: "tokenid", Value: bsonx.Int32(1)}, {Key: "respondblock", Value: bsonx.Int32(-1)}},
		},
	}
	_, err := mgm.Coll(&shared.ContributionData{}).Indexes().CreateMany(context.Background(), ctrbModel)
	if err != nil {
		return err
	}
	wdCtrbModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "contributor", Value: bsonx.Int32(1)}, {Key: "status", Value: bsonx.Int32(1)}, {Key: "respondtime", Value: bsonx.Int32(-1)}},
		},
	}
	_, err = mgm.Coll(&shared.WithdrawContributionData{}).Indexes().CreateMany(context.Background(), wdCtrbModel)
	if err != nil {
		return err
	}

	wdFeeCtrbModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "contributor", Value: bsonx.Int32(1)}, {Key: "status", Value: bsonx.Int32(1)}, {Key: "respondtime", Value: bsonx.Int32(-1)}},
		},
	}
	_, err = mgm.Coll(&shared.WithdrawContributionFeeData{}).Indexes().CreateMany(context.Background(), wdFeeCtrbModel)
	if err != nil {
		return err
	}

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
	_, err = mgm.Coll(&shared.TradeOrderData{}).Indexes().CreateMany(context.Background(), tradeOrderModel)
	if err != nil {
		return err
	}

	poolPairModel := []mongo.IndexModel{
		{
			Keys: bsonx.Doc{{Key: "pairid", Value: bsonx.Int32(1)}, {Key: "poolid", Value: bsonx.Int32(1)}},
		},
	}
	_, err = mgm.Coll(&shared.PoolPairData{}).Indexes().CreateMany(context.Background(), poolPairModel)
	if err != nil {
		return err
	}

	return nil
}

func DBCreateTokenIndex() error {
	tokenModel := []mongo.IndexModel{
		{
			Keys:    bsonx.Doc{{Key: "tokenid", Value: bsonx.Int32(1)}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := mgm.Coll(&shared.TokenInfoData{}).Indexes().CreateMany(context.Background(), tokenModel)
	if err != nil {
		return err
	}
	return nil
}
