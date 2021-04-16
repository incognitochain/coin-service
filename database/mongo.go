package database

import (
	"context"
	"encoding/base64"
	"log"
	"main/shared"
	"sort"
	"time"

	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
)

func ConnectDB(cfg *shared.Config) error {
	err := mgm.SetDefaultConfig(nil, cfg.MongoDB, options.Client().ApplyURI(cfg.MongoAddress))
	if err != nil {
		return err
	}
	_, cd, _, _ := mgm.DefaultConfigs()
	err = cd.Ping(context.Background(), nil)
	if err != nil {
		return err
	}
	log.Println("Database Connected!")
	return nil
}

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
			Keys: bsonx.Doc{{Key: "bucketid", Value: bsonx.Int32(1)}},
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

func DBSaveCoins(list []shared.CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
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
		_, err := mgm.Coll(&shared.CoinData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docs), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v2coins in %v", len(docs), time.Since(startTime))
	}
	if len(docsV1) > 0 {
		_, err := mgm.Coll(&shared.CoinDataV1{}).InsertMany(ctx, docsV1)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docsV1), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v1coins in %v", len(docsV1), time.Since(startTime))
	}
	return nil
}

func DBUpdateCoins(list []shared.CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
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

func DBGetCoinsByIndex(idx int, shardID int, tokenID string) (*shared.CoinData, error) {
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
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "otasecret": bson.M{operator.Eq: ""}, "tokenid": bson.M{operator.Eq: tokenID}, "coinidx": bson.M{operator.Gte: fromidx, operator.Lte: fromidx + limit}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		// Sort: bson.D{{Key: "coinidx", Value: 1}},
		// Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CoinIndex < list[j].CoinIndex })
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetCoinsByOTAKey(shardID int, tokenID, OTASecret string, fromidx, limit int64) ([]shared.CoinData, error) {
	startTime := time.Now()
	list := []shared.CoinData{}
	if limit == 0 {
		limit = 10000
	}
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "otasecret": bson.M{operator.Eq: OTASecret}, "tokenid": bson.M{operator.Eq: tokenID}, "coinidx": bson.M{operator.Gte: fromidx}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CoinIndex < list[j].CoinIndex })
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

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

func DBSaveUsedKeyimage(list []shared.KeyImageData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
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
	var listToCheck []string
	var kmsdata []shared.KeyImageData
	for _, v := range list {
		a, _ := base64.StdEncoding.DecodeString(v)
		listToCheck = append(listToCheck, base58.EncodeCheck(a))
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(listToCheck)+1)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"keyimage": bson.M{operator.In: listToCheck}, "shardid": bson.M{operator.Eq: shardID}}
	err := mgm.Coll(&shared.KeyImageData{}).SimpleFindWithCtx(ctx, &kmsdata, filter)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	for _, km := range listToCheck {
		found := false
		for _, rkm := range kmsdata {
			if km == rkm.KeyImage {
				found = true
				break
			}
		}
		result = append(result, found)
	}
	log.Printf("checked %v keyimages in %v", len(listToCheck), time.Since(startTime))
	return result, nil
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

func DBGetCoinV2PubkeyInfo(key string) (*shared.KeyInfoData, error) {
	var result shared.KeyInfoData
	filter := bson.M{"otakey": bson.M{operator.Eq: key}}
	err := mgm.Coll(&shared.KeyInfoDataV2{}).First(filter, &result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &shared.KeyInfoData{
				OTAKey: key,
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

func DBGetCoinV2OfShardCount(shardID int, tokenID string) int64 {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	doc := shared.CoinData{}
	count, err := mgm.Coll(&doc).CountDocuments(ctx, filter)
	if err != nil {
		log.Println(err)
		return -1
	}
	return count
}

func DBSavePendingTx(list []shared.CoinPendingData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.CoinPendingData{}).Drop(ctx)
	if err != nil {
		log.Println(err)
		return err
	}
	if len(list) > 0 {
		ctx, _ = context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, coin := range list {
			docs = append(docs, coin)
		}
		_, err = mgm.Coll(&shared.CoinPendingData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}

func DBSaveTokenInfo(list []shared.TokenInfoData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TokenInfoData{}).Drop(ctx)
	if err != nil {
		log.Println(err)
		return err
	}
	if len(list) > 0 {
		ctx, _ = context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, token := range list {
			docs = append(docs, token)
		}
		_, err = mgm.Coll(&shared.TokenInfoData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}

func DBGetTokenCount() (int64, error) {
	filter := bson.M{}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	c, err := mgm.Coll(&shared.TokenInfoData{}).Collection.CountDocuments(ctx, filter)
	if err != nil {
		return c, err
	}
	return c, nil
}

func DBGetTokenInfo() ([]shared.TokenInfoData, error) {
	list := []shared.TokenInfoData{}
	filter := bson.M{}
	err := mgm.Coll(&shared.TokenInfoData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DBGetPendingCoins() ([]string, error) {
	list := []shared.CoinPendingData{}
	filter := bson.M{}
	err := mgm.Coll(&shared.CoinPendingData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, v := range list {
		result = append(result, v.SerialNumber...)
	}
	return result, nil
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

func DBCheckTxsExist(txList []string, shardID int) ([]bool, error) {
	startTime := time.Now()
	var result []bool
	var kmsdata []shared.KeyImageData
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(txList)+1)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"txhash": bson.M{operator.In: txList}, "shardid": bson.M{operator.Eq: shardID}}
	err := mgm.Coll(&shared.KeyImageData{}).SimpleFindWithCtx(ctx, &kmsdata, filter)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	for _, km := range txList {
		found := false
		for _, rkm := range kmsdata {
			if km == rkm.TxHash {
				found = true
				break
			}
		}
		result = append(result, found)
	}
	log.Printf("checked %v keyimages in %v", len(txList), time.Since(startTime))
	return result, nil
}

func DBGetSubmittedOTAKeys(bucketID int, offset int64) ([]shared.SubmittedOTAKeyData, error) {
	var result []shared.SubmittedOTAKeyData
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"bucketid": bson.M{operator.Eq: bucketID}}
	err := mgm.Coll(&shared.SubmittedOTAKeyData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort: bson.D{{"created_at", 1}},
		Skip: &offset,
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return result, nil
}

func DBSaveSubmittedOTAKeys(keys []shared.SubmittedOTAKeyData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	if len(keys) > 0 {
		ctx, _ = context.WithTimeout(context.Background(), time.Duration(len(keys))*shared.DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, key := range keys {
			key.Creating()
			key.Saving()
			docs = append(docs, key)
		}
		_, err := mgm.Coll(&shared.SubmittedOTAKeyData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func DBGetBucketStats(bucketSize int) (map[int]uint64, error) {
	result := make(map[int]uint64)
	d := mgm.Coll(&shared.SubmittedOTAKeyData{})
	for i := 0; i < bucketSize; i++ {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(10)*shared.DB_OPERATION_TIMEOUT)
		filter := bson.M{"bucketid": bson.M{operator.Eq: i}}
		count, err := d.CountDocuments(ctx, filter)
		if err != nil {
			return nil, err
		}
		result[i] = uint64(count)
	}

	return result, nil
}

func DBSaveCoinsUnfinalized(list []shared.CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
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
		_, err := mgm.Coll(&shared.CoinDataUnfinalized{}).InsertMany(ctx, docs)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docs), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v2coins in %v", len(docs), time.Since(startTime))
	}
	if len(docsV1) > 0 {
		_, err := mgm.Coll(&shared.CoinDataV1Unfinalized{}).InsertMany(ctx, docsV1)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docsV1), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v1coins in %v", len(docsV1), time.Since(startTime))
	}
	return nil
}

func DBSaveKeyimageUnfinalized(list []shared.KeyImageData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, km := range list {
		docs = append(docs, km)
	}
	_, err := mgm.Coll(&shared.KeyImageDataUnfinalized{}).InsertMany(ctx, docs)
	if err != nil {
		log.Printf("failed to insert %v keyimages in %v", len(list), time.Since(startTime))
		return err
	}
	log.Printf("inserted %v keyimages in %v", len(list), time.Since(startTime))
	return nil
}
