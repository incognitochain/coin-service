package database

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBSavePDEState(state string) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(2)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.PDEStateData{}).Drop(context.Background())
	if err != nil {
		log.Println(err)
		return err
	}
	var doc interface{}
	newState := shared.NewPDEStateData(state)
	doc = newState
	_, err = mgm.Coll(&shared.PDEStateData{}).InsertOne(ctx, doc)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func DBGetPDEState() (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(2)*shared.DB_OPERATION_TIMEOUT)
	var result shared.PDEStateData
	err := mgm.Coll(&shared.PDEStateData{}).FirstWithCtx(ctx, bson.M{}, &result)
	if err != nil {
		log.Println(err)
		return "", err
	}
	return result.State, nil
}

func DBSaveTxTrade(list []shared.TradeData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.TradeData{}).InsertMany(ctx, docs)
	if err != nil {
		return err
	}
	return nil
}

// func DBUpdateTxTrade(list []shared.TradeData) error {
// 	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
// 	docs := []interface{}{}
// 	for _, tx := range list {
// 		update := bson.M{
// 			"$set": tx,
// 		}
// 		docs = append(docs, update)
// 	}
// 	for idx, doc := range docs {
// 		ctx, _ := context.WithTimeout(context.Background(), 1*shared.DB_OPERATION_TIMEOUT)
// 		filter := bson.M{""}
// 		_, err := mgm.Coll(&shared.TradeData{}).FindOneAndUpdate(ctx,filter,)

// 		// _, err := mgm.Coll(&shared.TradeData{}).UpdateByID(ctx, list[idx].GetID(), doc)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

func DBGetTxTrade(respondList []string) ([]shared.TradeData, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(respondList)+1)*shared.DB_OPERATION_TIMEOUT)
	result := []shared.TradeData{}

	filter := bson.M{"respondtx": bson.M{operator.In: respondList}}
	err := mgm.Coll(&shared.TradeData{}).SimpleFindWithCtx(ctx, &result, filter)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return result, nil
}

func DBGetTxTradeRespond(pubkey string, limit int64, offset int64) ([]shared.TxData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	metas := []string{strconv.Itoa(metadata.PDECrossPoolTradeResponseMeta), strconv.Itoa(metadata.PDETradeResponseMeta)}
	var result []shared.TxData
	filter := bson.M{"pubkeyreceivers": bson.M{operator.Eq: pubkey}, "metatype": bson.M{operator.In: metas}}
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
