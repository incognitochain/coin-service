package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func connectDB() error {
	err := mgm.SetDefaultConfig(nil, "coins", options.Client().ApplyURI(serviceCfg.MongoAddress))
	if err != nil {
		return err
	}
	log.Println("Database Connected!")
	return nil
}

func DBCreateCoinV1Index() error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*DB_OPERATION_TIMEOUT)
	indexName, err := mgm.Coll(&CoinDataV1{}).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"beaconheight": -1,
		},
		// Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Printf("failed to indexs coins in %v", time.Since(startTime))
		return err
	}
	log.Println("indexName", indexName)
	log.Printf("success indexs coins in %v", time.Since(startTime))
	return nil
}

func DBSaveCoins(list []CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	docsV1 := []interface{}{}
	for _, coin := range list {
		if coin.CoinVersion == 2 {
			docs = append(docs, coin)
		} else {
			docsV1 = append(docsV1, coin)
		}
	}
	if len(docs) > 0 {
		_, err := mgm.Coll(&CoinData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docs), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v2coins in %v", len(docs), time.Since(startTime))

	}
	if len(docsV1) > 0 {
		_, err := mgm.Coll(&CoinDataV1{}).InsertMany(ctx, docsV1)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docsV1), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v1coins in %v", len(docsV1), time.Since(startTime))
	}
	return nil
}

func DBUpdateCoins(list []CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, coin := range list {
		update := bson.M{
			"$set": coin,
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		fmt.Println(list[idx].GetID())
		_, err := mgm.Coll(&CoinData{}).UpdateByID(ctx, list[idx].GetID(), doc)
		if err != nil {
			log.Printf("failed to update %v coins in %v", len(list), time.Since(startTime))
			return err
		}
	}
	log.Printf("updated %v coins in %v", len(list), time.Since(startTime))
	return nil
}

func DBGetCoinsByIndex(idx int, shardID int, tokenID string) (*CoinData, error) {
	var result CoinData
	filter := bson.M{"coinidx": bson.M{operator.Eq: idx}, "shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&CoinData{}).First(filter, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func DBGetCoinsByOTAKey(OTASecret string) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	temp, _, err := base58.DecodeCheck(OTASecret)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"otasecret": bson.M{operator.Eq: hex.EncodeToString(temp)}}
	err = mgm.Coll(&CoinData{}).SimpleFind(&list, filter)
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetCoinsByOTAKeyAndHeight(tokenID, OTASecret string, fromHeight int, toHeight int) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"otasecret": bson.M{operator.Eq: OTASecret}, "beaconheight": bson.M{operator.Gte: fromHeight, operator.Lte: toHeight}, "tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&CoinData{}).SimpleFind(&list, filter)
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetUnknownCoinsFromBeaconHeight(beaconHeight uint64) ([]CoinData, error) {
	limit := int64(500)
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"beaconheight": bson.M{operator.Gte: beaconHeight}, "otasecret": bson.M{operator.Eq: ""}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&CoinData{}).SimpleFindWithCtx(ctx, &list, filter, &options.FindOptions{
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetUnknownCoinsFromCoinIndexWithLimit(index uint64, isPRV bool, limit int64) ([]CoinData, error) {
	tokenID := common.PRVCoinID.String()
	if !isPRV {
		tokenID = common.ConfidentialAssetID.String()
	}
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"coinidx": bson.M{operator.Gte: index}, "otasecret": bson.M{operator.Eq: ""}, "tokenid": bson.M{operator.Eq: tokenID}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&CoinData{}).SimpleFindWithCtx(ctx, &list, filter, &options.FindOptions{
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetCoinV1ByPubkey(tokenID, pubkey string, fromIdx int, toIdx int) ([]CoinData, error) {
	startTime := time.Now()
	totalIdxs := toIdx - fromIdx
	if totalIdxs < 0 {
		return nil, errors.New("invalid from/to index")
	}
	if totalIdxs/COINS_GET_PER_DBREQUEST < 5 {
		list := []CoinData{}
		filter := bson.M{"coinpubkey": bson.M{operator.Eq: pubkey}, "coinidx": bson.M{operator.Gte: fromIdx, operator.Lte: toIdx}, "tokenid": bson.M{operator.Eq: tokenID}}
		err := mgm.Coll(&CoinDataV1{}).SimpleFind(&list, filter)
		if err != nil {
			return nil, err
		}
		log.Printf("found %v coins in %v", len(list), time.Since(startTime))
		return list, err
	}
	log.Println("large db request")
	collectCh := make(chan []CoinData, 5)
	finalList := []CoinData{}
	from := fromIdx
	to := from + COINS_GET_PER_DBREQUEST
	if to > toIdx {
		to = toIdx
	}
	var wg sync.WaitGroup
	requestCount := 0
	for {
		wg.Add(1)
		requestCount++
		go func(f, t int) {
		get:
			log.Println("get coins", f, t)
			list := []CoinData{}
			filter := bson.M{"coinpubkey": bson.M{operator.Eq: pubkey}, "coinidx": bson.M{operator.Gte: f, operator.Lte: t}, "tokenid": bson.M{operator.Eq: tokenID}}
			err := mgm.Coll(&CoinDataV1{}).SimpleFind(&list, filter)
			if err != nil {
				log.Println(err)
				goto get
			}
			log.Println("len(list)", len(list), len(collectCh))
			collectCh <- list
			wg.Done()
		}(from, to)
		if requestCount%5 == 0 || to+COINS_GET_PER_DBREQUEST >= toIdx {
			wg.Wait()
			log.Println("collect coins")
			close(collectCh)
			for list := range collectCh {
				finalList = append(finalList, list...)
			}
			collectCh = make(chan []CoinData, 5)
		}
		from = to
		if from == toIdx {
			break
		}
		to = from + COINS_GET_PER_DBREQUEST
		if to > toIdx {
			to = toIdx
		}
	}
	log.Printf("found %v coins in %v", len(finalList), time.Since(startTime))
	return finalList, nil
}

func DBGetCoinsOTAStat() error {
	return nil
}

func DBSaveUsedKeyimage(list []KeyImageData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, coin := range list {
		docs = append(docs, coin)
	}
	_, err := mgm.Coll(&list[0]).InsertMany(ctx, docs)
	if err != nil {
		log.Printf("failed to insert %v keyimages in %v", len(list), time.Since(startTime))
		return err
	}
	log.Printf("inserted %v keyimages in %v", len(list), time.Since(startTime))
	return nil
}

func DBCheckKeyimagesUsed(list []string, shardID int) ([]bool, error) {
	startTime := time.Now()
	var result []bool

	var kmsdata []KeyImageData
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*DB_OPERATION_TIMEOUT)
	filter := bson.M{"keyimage": bson.M{operator.In: list}}
	err := mgm.Coll(&KeyImageData{}).SimpleFindWithCtx(ctx, &kmsdata, filter)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	for _, km := range list {
		found := false
		for _, rkm := range kmsdata {
			if km == rkm.KeyImage {
				found = true
				break
			}
		}
		result = append(result, found)
	}
	// for _, keyImage := range list {
	// 	filter := bson.M{"keyimage": bson.M{operator.Eq: keyImage}}
	// 	err := mgm.Coll(&KeyImageData{}).First(filter, &kmdata)
	// 	if err != nil {
	// 		log.Println(keyImage, err)
	// 		result = append(result, false)
	// 		continue
	// 	}
	// 	result = append(result, true)
	// }

	log.Printf("checked %v keyimages in %v", len(list), time.Since(startTime))
	return result, nil
}

func DBUpdateCoinV1PubkeyInfo(list map[string]map[string]uint64) error {
	pubkeys := []string{}
	lenList := len(list)
	for pubkey, _ := range list {
		pubkeys = append(pubkeys, pubkey)
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*DB_OPERATION_TIMEOUT)
	KeyInfoDatas := []KeyInfoData{}
	filter := bson.M{"pubkey": bson.M{operator.In: pubkeys}}
	err := mgm.Coll(&KeyInfoData{}).SimpleFindWithCtx(ctx, &KeyInfoDatas, filter)
	if err != nil {
		log.Println(err)
		return err
	}
	keysToInsert := []KeyInfoData{}
	keysToUpdate := []KeyInfoData{}
	for _, keyInfo := range KeyInfoDatas {
		ki, ok := list[keyInfo.Pubkey]
		change := false
		for token, idx := range ki {
			if _, exist := keyInfo.CoinV1StartIndex[token]; !exist {
				keyInfo.CoinV1StartIndex[token] = idx
				change = true
			}
		}

		if ok {
			delete(list, keyInfo.Pubkey)
		}
		if change {
			keysToUpdate = append(keysToUpdate, keyInfo)
		}
	}

	for key, tokens := range list {
		newKeyInfo := NewKeyInfoData(key, "", tokens, nil)
		keysToInsert = append(keysToInsert, *newKeyInfo)
	}
	if len(keysToInsert) > 0 {
		ctx, _ = context.WithTimeout(context.Background(), time.Duration(len(keysToInsert))*DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, key := range keysToInsert {
			docs = append(docs, key)
		}
		_, err = mgm.Coll(&KeyInfoData{}).InsertMany(ctx, docs)
		if err != nil {
			return err
		}
	}
	if len(keysToUpdate) > 0 {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(keysToUpdate))*DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, key := range keysToUpdate {
			update := bson.M{
				"$set": key,
			}
			docs = append(docs, update)
		}
		for idx, doc := range docs {
			_, err := mgm.Coll(&KeyInfoData{}).UpdateByID(ctx, keysToUpdate[idx].GetID(), doc)
			if err != nil {
				return err
			}
		}
	}

	log.Println("update %v keys info successful", lenList)
	return nil
}

func DBGetCoinPubkeyInfo(key string) (*KeyInfoData, error) {
	var result KeyInfoData
	filter := bson.M{"pubkey": bson.M{operator.Eq: key}}
	err := mgm.Coll(&KeyInfoData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func DBGetCoinV1OfShardCount(shardID int, tokenID string) int64 {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*DB_OPERATION_TIMEOUT)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	doc := CoinDataV1{}
	count, err := mgm.Coll(&doc).CountDocuments(ctx, filter)
	if err != nil {
		log.Println(err)
		return -1
	}
	return count
}

func DBGetCoinV2OfShardCount(shardID int, tokenID string) int64 {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*DB_OPERATION_TIMEOUT)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	doc := CoinData{}
	count, err := mgm.Coll(&doc).CountDocuments(ctx, filter)
	if err != nil {
		log.Println(err)
		return -1
	}
	return count
}
