package database

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBSaveTXs(list []shared.TxData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.TxData{}).InsertMany(ctx, docs)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func DBUpdateTxPubkeyReceiver(txHashes []string, pubKey string) error {
	docs := []interface{}{}
	for _ = range txHashes {
		update := bson.M{
			"$addToSet": bson.M{"pubkeyreceivers": pubKey},
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		filter := bson.M{"txhash": bson.M{operator.Eq: txHashes[idx]}}
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(1)*shared.DB_OPERATION_TIMEOUT)
		_, err := mgm.Coll(&shared.TxData{}).UpdateOne(ctx, filter, doc)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBGetSendTxByKeyImages(keyimages []string) ([]shared.TxData, error) {
	var result []shared.TxData
	filter := bson.M{"keyimages": bson.M{operator.In: keyimages}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(keyimages))*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort: bson.D{{"locktime", -1}},
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetReceiveTxByPubkey(pubkey string, tokenID string, limit int64, offset int64) ([]shared.TxData, error) {
	var result []shared.TxData
	if limit == 0 {
		limit = int64(10000)
	}
	filter := bson.M{"tokenid": bson.M{operator.Eq: tokenID}, "pubkeyreceivers": bson.M{operator.Eq: pubkey}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"locktime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetTxByHash(txHashes []string) ([]shared.TxData, error) {
	var result []shared.TxData
	var resultFn []shared.TxData
	filter := bson.M{"txhash": bson.M{operator.In: txHashes}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(txHashes)+1)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(ctx, &result, filter)
	if err != nil {
		return nil, err
	}
	for _, hash := range txHashes {
		for _, v := range result {
			if v.TxHash == hash {
				resultFn = append(resultFn, v)
				break
			}
		}
	}

	return resultFn, nil
}
