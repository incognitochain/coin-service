package database

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBUpdateCoins(list []shared.CoinData, ctx context.Context) error {
	startTime := time.Now()
	docs := []interface{}{}
	for _, coin := range list {
		update := bson.M{
			"$set": coin,
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		_, err := mgm.Coll(&shared.CoinData{}).UpdateByID(ctx, list[idx].GetID(), doc)
		if err != nil {
			log.Printf("failed to update %v coins in %v", len(list), time.Since(startTime))
			return err
		}
	}
	log.Printf("updated %v coins in %v", len(list), time.Since(startTime))
	return nil
}

func DBGetCoinsByIndex(idx uint64, shardID int, tokenID string) (*shared.CoinData, error) {
	var result shared.CoinData
	filter := bson.M{"coinidx": bson.M{operator.Eq: idx}, "shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&shared.CoinData{}).First(filter, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func DBGetUnknownCoinsV2(shardID int, tokenID string, fromidx, limit int64) ([]shared.CoinData, error) {
	startTime := time.Now()
	list := []shared.CoinData{}
	if limit == 0 {
		limit = 10000
	}
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}, "coinidx": bson.M{operator.Gte: fromidx}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	newList := filterByIndexedCoins(list)
	sort.Slice(newList, func(i, j int) bool { return newList[i].CoinIndex < newList[j].CoinIndex })
	log.Printf("found %v/%v fromidx %v shard %v coins in %v", len(newList), len(list), fromidx, shardID, time.Since(startTime))
	return newList, err
}

func DBGetUnknownCoinsV21(shardID int, tokenID string, fromidx, limit int64) ([]shared.CoinData, error) {
	startTime := time.Now()
	list := []shared.CoinData{}
	if limit == 0 {
		limit = 10000
	}
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "otasecret": bson.M{operator.Eq: ""}, "tokenid": bson.M{operator.Eq: tokenID}, "coinidx": bson.M{operator.Gte: fromidx}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(list, func(i, j int) bool { return list[i].CoinIndex < list[j].CoinIndex })
	log.Printf("found %v fromidx %v shard %v coins in %v", len(list), fromidx, shardID, time.Since(startTime))
	return list, err
}

func DBGetUnknownCoinsV22(shardID int, tokenID string, fromidx, toidx, limit int64) ([]shared.CoinData, error) {
	list := []shared.CoinData{}
	if limit == 0 {
		limit = 10000
	}
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "otasecret": bson.M{operator.Eq: ""}, "tokenid": bson.M{operator.Eq: tokenID}, "coinidx": bson.M{operator.Gte: fromidx, operator.Lte: toidx}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return list, err
}

func DBGetCoinsByOTAKey(shardID int, tokenID, OTASecret string, offset, limit int64) ([]shared.CoinData, error) {
	startTime := time.Now()
	list := []shared.CoinData{}
	if limit == 0 {
		limit = 10000
	}
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "otasecret": bson.M{operator.Eq: OTASecret}, "realtokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CoinIndex < list[j].CoinIndex })
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBUpdateCoinV2PubkeyInfo(list map[string]map[string]shared.CoinInfo) error {
	otakeys := []string{}
	lenList := len(list)
	for otakey, _ := range list {
		otakeys = append(otakeys, otakey)
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	KeyInfoDatas := []shared.KeyInfoData{}
	filter := bson.M{"otakey": bson.M{operator.In: otakeys}}
	err := mgm.Coll(&shared.KeyInfoDataV2{}).SimpleFindWithCtx(ctx, &KeyInfoDatas, filter)
	if err != nil {
		log.Println(err)
		return err
	}
	keysToInsert := []shared.KeyInfoData{}
	keysToUpdate := []shared.KeyInfoData{}
	for _, keyInfo := range KeyInfoDatas {
		ki, ok := list[keyInfo.OTAKey]
		for token, idx := range ki {
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
			delete(list, keyInfo.OTAKey)
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
			key.Creating()
			docs = append(docs, key)
		}
		_, err = mgm.Coll(&shared.KeyInfoDataV2{}).InsertMany(ctx, docs)
		if err != nil {
			return err
		}
	}
	if len(keysToUpdate) > 0 {
		docs := []interface{}{}
		for _, key := range keysToUpdate {
			key.Saving()
			update := bson.M{
				"$set": key,
			}
			docs = append(docs, update)
		}
		for idx, doc := range docs {
			ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
			_, err := mgm.Coll(&shared.KeyInfoDataV2{}).UpdateByID(ctx, keysToUpdate[idx].GetID(), doc)
			if err != nil {
				return err
			}
		}
	}

	log.Printf("update %v keys info successful\n", lenList)
	return nil
}

func DBGetCoinV2PubkeyInfo(key string) (*shared.KeyInfoData, error) {
	var result shared.KeyInfoData
	filter := bson.M{"pubkey": bson.M{operator.Eq: key}}
	err := mgm.Coll(&shared.KeyInfoDataV2{}).First(filter, &result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &shared.KeyInfoData{
				Pubkey:    key,
				CoinIndex: make(map[string]shared.CoinInfo),
			}, nil
		}
		return nil, err
	}
	return &result, nil
}

func DBGetCoinV2OfShardCount(shardID int, tokenID string) (int64, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	doc := shared.CoinData{}
	count, err := mgm.Coll(&doc).CountDocuments(ctx, filter)
	if err != nil {
		return -1, err
	}
	return count, nil
}

func DBGetCoinV2OfOTAkeyCount(shardID int, tokenID, otakey string) (uint64, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(500)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "realtokenid": bson.M{operator.Eq: tokenID}, "otasecret": bson.M{operator.Eq: otakey}}
	doc := shared.CoinData{}
	count, err := mgm.Coll(&doc).CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return uint64(count), nil
}

func DBGetAllCoinV2OfOTAkey(shardID int, tokenID, otakey string, offset int64) ([]shared.CoinData, error) {
	var coinList []shared.CoinData
	limit := int64(10000)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}, "otasecret": bson.M{operator.Eq: otakey}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&coinList, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Limit: &limit,
		Skip:  &offset,
	})
	if err != nil {
		return nil, err
	}
	return coinList, nil
}

func DBGetLastCoinV2OfOTAkey(shardID int, tokenID, otakey string) (uint64, error) {
	var coinList []shared.CoinData
	limit := int64(1)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "realtokenid": bson.M{operator.Eq: tokenID}, "otasecret": bson.M{operator.Eq: otakey}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&coinList, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", -1}},
		Limit: &limit,
	})
	if err != nil {
		return 0, err
	}
	if len(coinList) == 0 {
		return 0, nil
	}
	return coinList[0].CoinIndex, nil
}

func DBGetStartCoinV2OfOTAkey(shardID int, tokenID, otakey string) (uint64, error) {
	var coinList []shared.CoinData
	limit := int64(1)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "realtokenid": bson.M{operator.Eq: tokenID}, "otasecret": bson.M{operator.Eq: otakey}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&coinList, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Limit: &limit,
	})
	if err != nil {
		return 0, err
	}
	if len(coinList) == 0 {
		return 0, nil
	}
	return coinList[0].CoinIndex, nil
}

func DBGetTxV2ByPubkey(pubkeys []string) ([]shared.TxData, []string, error) {
	var result []shared.TxData
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(pubkeys)+1)*shared.DB_OPERATION_TIMEOUT)
	coinDatas := []shared.CoinData{}
	filter := bson.M{"coinpubkey": bson.M{operator.In: pubkeys}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFindWithCtx(ctx, &coinDatas, filter)
	if err != nil {
		log.Println(err)
		return nil, nil, err
	}

	txToGet := []string{}
	pubkeyTxs := []string{}
	for _, coinData := range coinDatas {
		txToGet = append(txToGet, coinData.TxHash)
	}
	for _, key := range pubkeys {
		txHash := ""
		for _, coin := range coinDatas {
			if coin.CoinPubkey == key {
				txHash = coin.TxHash
				break
			}
		}
		if txHash != "" {
			txToGet = append(txToGet, txHash)
		}
		pubkeyTxs = append(pubkeyTxs, txHash)
	}
	result, err = DBGetTxByHash(txToGet)
	if err != nil {
		log.Println(err)
		return nil, nil, err
	}

	return result, pubkeyTxs, nil
}

func filterByIndexedCoins(coins []shared.CoinData) []shared.CoinData {
	var result []shared.CoinData
	for _, v := range coins {
		if v.OTASecret == "" {
			result = append(result, v)
		}
	}
	return result
}

func DBGetNFTByOTAKey(shardID int, OTASecret string, offset, limit int64) (map[string][]shared.CoinData, error) {
	result := make(map[string][]shared.CoinData)
	startTime := time.Now()
	list := []shared.CoinData{}
	if limit == 0 {
		limit = 10000
	}
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "otasecret": bson.M{operator.Eq: OTASecret}, "isnft": bson.M{operator.Eq: true}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CoinIndex < list[j].CoinIndex })
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	for _, coin := range list {
		result[coin.RealTokenID] = append(result[coin.RealTokenID], coin)
	}
	return result, err
}

func DBGetCoinsV2ByShardID(shardID int, tokenID string, limit, offset int64) ([]shared.CoinData, error) {
	var result []shared.CoinData
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&result, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	return result, err
}

func DBGetAllAccessCoin(otakey string) ([]shared.CoinData, error) {
	var coinList []shared.CoinData
	// limit := int64(10000)
	filter := bson.M{"tokenid": bson.M{operator.Eq: common.PdexAccessCoinID.String()}, "otasecret": bson.M{operator.Eq: otakey}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&coinList, filter, &options.FindOptions{
		Sort: bson.D{{"coinidx", 1}},
		// Limit: &limit,
		// Skip:  &offset,
	})
	if err != nil {
		return nil, err
	}
	return coinList, nil
}
