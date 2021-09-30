package database

import (
	"context"
	"encoding/json"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBGetDefaultPool() (map[string]struct{}, error) {
	var datas []shared.ClientAssistantData
	var list []string
	result := make(map[string]struct{})
	filter := bson.M{"dataname": bson.M{operator.Eq: "defaultpools"}}
	err := mgm.Coll(&shared.ClientAssistantData{}).SimpleFind(&datas, filter)
	if err != nil {
		return nil, err
	}
	if len(datas) == 0 {
		return nil, nil
	}
	err = json.Unmarshal([]byte(datas[0].Data), &list)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		result[v] = struct{}{}
	}
	return result, nil
}

func DBGetTop10PairHighestCap() ([]shared.PairRanking, error) {
	limit := int64(10)
	var results []shared.PairRanking
	filter := bson.M{}
	err := mgm.Coll(&shared.PairRanking{}).SimpleFind(&results, filter, &options.FindOptions{
		Sort:  bson.D{{"value", -1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func DBGetTokenPrice(tokenID string) (*shared.TokenPrice, error) {
	var result shared.TokenPrice
	filter := bson.M{"tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&shared.TokenPrice{}).First(filter, &result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func DBGetBridgeTokens() ([]shared.TokenInfoData, error) {
	var results []shared.TokenInfoData
	filter := bson.M{"isbridge": bson.M{operator.Eq: true}}
	err := mgm.Coll(&shared.TokenInfoData{}).SimpleFind(&results, filter)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func DBSaveTokenPrice(list []shared.TokenPrice) error {
	docs := []interface{}{}
	for _, tx := range list {
		update := bson.M{
			"$set": tx,
		}
		docs = append(docs, update)
	}
	for idx, v := range list {
		filter := bson.M{"tokenid": bson.M{operator.Eq: v.TokenID}}
		_, err := mgm.Coll(&shared.TokenPrice{}).UpdateOne(context.Background(), filter, docs[idx], mgm.UpsertTrueOption())
		if err != nil {
			writeErr, ok := err.(mongo.WriteException)
			if !ok {
				panic(err)
			}
			if !writeErr.HasErrorCode(11000) {
				panic(err)
			}
		}
	}
	return nil
}

func DBSavePairRanking(list []shared.PairRanking) error {
	docs := []interface{}{}
	for _, tx := range list {
		update := bson.M{
			"$set": tx,
		}
		docs = append(docs, update)
	}
	for idx, v := range list {
		filter := bson.M{"pairid": bson.M{operator.Eq: v.PairID}}
		_, err := mgm.Coll(&shared.PairRanking{}).UpdateOne(context.Background(), filter, docs[idx], mgm.UpsertTrueOption())
		if err != nil {
			writeErr, ok := err.(mongo.WriteException)
			if !ok {
				panic(err)
			}
			if !writeErr.HasErrorCode(11000) {
				panic(err)
			}
		}
	}
	return nil
}

func DBSaveTokenMkCap(list []shared.TokenMarketCap) error {
	docs := []interface{}{}
	for _, tx := range list {
		update := bson.M{
			"$set": tx,
		}
		docs = append(docs, update)
	}
	for idx, v := range list {
		filter := bson.M{"symbol": bson.M{operator.Eq: v.TokenSymbol}}
		_, err := mgm.Coll(&shared.TokenMarketCap{}).UpdateOne(context.Background(), filter, docs[idx], mgm.UpsertTrueOption())
		if err != nil {
			writeErr, ok := err.(mongo.WriteException)
			if !ok {
				panic(err)
			}
			if !writeErr.HasErrorCode(11000) {
				panic(err)
			}
		}
	}
	return nil
}
