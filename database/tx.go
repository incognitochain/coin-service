package database

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson"
)

func DBSaveTXs(list []shared.TxData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.TxData{}).InsertMany(ctx, docs)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func DBUpdateTxPubkeySend(list []shared.TxData, pubKey string) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.PubKeySend = pubKey
		update := bson.M{
			"$set": tx,
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		_, err := mgm.Coll(&shared.TxData{}).UpdateByID(ctx, list[idx].GetID(), doc)
		if err != nil {
			return err
		}
	}
	return nil
}
func DBUpdateTxPubkeyReceiver(list []shared.TxData, pubKey string) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _ = range list {
		update := bson.M{
			"$addToSet": bson.M{"pubkeyreceivers": pubKey},
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		_, err := mgm.Coll(&shared.TxData{}).UpdateByID(ctx, list[idx].GetID(), doc)
		if err != nil {
			return err
		}
	}
	return nil
}
