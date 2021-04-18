package database

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBGetCoinV1ByPubkey(tokenID, pubkey string, offset int64, limit int64) ([]shared.CoinData, error) {
	startTime := time.Now()
	if limit == 0 {
		limit = int64(10000)
	}
	list := []shared.CoinData{}
	filter := bson.M{"coinpubkey": bson.M{operator.Eq: pubkey}, "tokenid": bson.M{operator.Eq: tokenID}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.CoinDataV1{}).SimpleFindWithCtx(ctx, &list, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBUpdateCoinV1PubkeyInfo(list map[string]map[string]shared.CoinInfo) error {
	pubkeys := []string{}
	lenList := len(list)
	for pubkey, _ := range list {
		pubkeys = append(pubkeys, pubkey)
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	KeyInfoDatas := []shared.KeyInfoData{}
	filter := bson.M{"pubkey": bson.M{operator.In: pubkeys}}
	err := mgm.Coll(&shared.KeyInfoData{}).SimpleFindWithCtx(ctx, &KeyInfoDatas, filter)
	if err != nil {
		log.Println(err)
		return err
	}
	keysToInsert := []shared.KeyInfoData{}
	keysToUpdate := []shared.KeyInfoData{}
	for _, keyInfo := range KeyInfoDatas {
		ki, ok := list[keyInfo.Pubkey]
		for token, idx := range ki {
			if len(keyInfo.CoinIndex) == 0 {
				keyInfo.CoinIndex = make(map[string]shared.CoinInfo)
			}
			if _, exist := keyInfo.CoinIndex[token]; !exist {
				keyInfo.CoinIndex[token] = idx
			} else {
				info := keyInfo.CoinIndex[token]
				info.End = idx.End
				info.Total = info.Total + idx.Total
				keyInfo.CoinIndex[token] = info
			}
		}

		if ok {
			delete(list, keyInfo.Pubkey)
		}
		keysToUpdate = append(keysToUpdate, keyInfo)
	}

	for key, tokens := range list {
		newKeyInfo := shared.NewKeyInfoData(key, "", tokens)
		keysToInsert = append(keysToInsert, *newKeyInfo)
	}
	if len(keysToInsert) > 0 {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(keysToInsert)+10)*shared.DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, key := range keysToInsert {
			docs = append(docs, key)
		}
		_, err = mgm.Coll(&shared.KeyInfoData{}).InsertMany(ctx, docs)
		if err != nil {
			return err
		}
	}
	if len(keysToUpdate) > 0 {
		docs := []interface{}{}
		for _, key := range keysToUpdate {
			update := bson.M{
				"$set": key,
			}
			docs = append(docs, update)
		}
		for idx, doc := range docs {
			ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
			_, err := mgm.Coll(&shared.KeyInfoData{}).UpdateByID(ctx, keysToUpdate[idx].GetID(), doc)
			if err != nil {
				return err
			}
		}
	}

	log.Printf("update %v keys info successful\n", lenList)
	return nil
}

func DBGetCoinV1ByIndexes(indexes []uint64, shardID int, tokenID string) ([]shared.CoinDataV1, error) {
	startTime := time.Now()
	var result []shared.CoinDataV1
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(indexes)+1)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"coinidx": bson.M{operator.In: indexes}, "shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&shared.CoinDataV1{}).SimpleFindWithCtx(ctx, &result, filter)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Printf("found %v coinV1 in %v", len(result), time.Since(startTime))
	return result, nil
}

func DBGetCoinV1PubkeyInfo(key string) (*shared.KeyInfoData, error) {
	var result shared.KeyInfoData
	filter := bson.M{"pubkey": bson.M{operator.Eq: key}}
	err := mgm.Coll(&shared.KeyInfoData{}).First(filter, &result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &shared.KeyInfoData{
				Pubkey: key,
			}, nil
		}
		return nil, err
	}
	return &result, nil
}

func DBGetCoinV1OfShardCount(shardID int, tokenID string) int64 {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	doc := shared.CoinDataV1{}
	count, err := mgm.Coll(&doc).CountDocuments(ctx, filter)
	if err != nil {
		log.Println(err)
		return -1
	}
	return count
}
