package database

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBSaveTXs(list []shared.TxData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.TxData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
	if err != nil {
		writeErr, ok := err.(mongo.BulkWriteException)
		if !ok {
			panic(err)
		}
		if ctx.Err() != nil {
			t, k := ctx.Deadline()
			log.Println("context error:", ctx.Err(), t, k)
		}
		er := writeErr.WriteErrors[0]
		if er.WriteError.Code != 11000 {
			panic(err)
		} else {
			for _, v := range docs {
				ctx, _ := context.WithTimeout(context.Background(), time.Duration(2)*shared.DB_OPERATION_TIMEOUT)
				_, err = mgm.Coll(&shared.TxData{}).InsertOne(ctx, v)
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
		}
	}

	return nil
}

func DBUpdateTxPubkeyReceiverAndTokenID(txHashes []string, pubKey, tokenID string, ctx context.Context) error {
	docs := []interface{}{}
	for _ = range txHashes {
		update := bson.M{
			"$addToSet": bson.M{"pubkeyreceivers": pubKey},
			"$set":      bson.M{"realtokenid": tokenID},
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		filter := bson.M{"txhash": bson.M{operator.Eq: txHashes[idx]}}
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
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(keyimages)+5)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort: bson.D{{"locktime", -1}},
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetReceiveTxByPubkey(pubkey string, tokenID string, txversion int, limit int64, offset int64) ([]shared.TxData, error) {
	var result []shared.TxData
	if limit == 0 {
		limit = int64(10000)
	}
	filter := bson.M{}
	if txversion == 1 {
		filter = bson.M{"tokenid": bson.M{operator.Eq: tokenID}, "pubkeyreceivers": bson.M{operator.Eq: pubkey}, "txversion": bson.M{operator.Eq: txversion}}
	} else {
		filter = bson.M{"realtokenid": bson.M{operator.Eq: tokenID}, "pubkeyreceivers": bson.M{operator.Eq: pubkey}, "txversion": bson.M{operator.Eq: txversion}}
	}
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

func DBIsTokenNFT(tokenid string) bool {
retry:
	filter := bson.M{"tokenid": bson.M{operator.Eq: tokenid}, "isnft": bson.M{operator.Eq: true}}
	result := mgm.Coll(&shared.TxData{}).FindOne(context.Background(), filter)
	if result.Err() != nil {
		if result.Err() == mongo.ErrNoDocuments {
			return false
		} else {
			log.Println(result.Err())
			time.Sleep(1 * time.Second)
			goto retry
		}
	}
	return true
}

func DBGetCountTxByPubkey(pubkey string, tokenID string, txversion int) (int64, error) {
	filter := bson.M{}
	if txversion == 1 {
		filter = bson.M{"tokenid": bson.M{operator.Eq: tokenID}, "pubkeyreceivers": bson.M{operator.Eq: pubkey}, "txversion": bson.M{operator.Eq: txversion}}
	} else {
		filter = bson.M{"realtokenid": bson.M{operator.Eq: tokenID}, "pubkeyreceivers": bson.M{operator.Eq: pubkey}, "txversion": bson.M{operator.Eq: txversion}}
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(100)*shared.DB_OPERATION_TIMEOUT)

	count, err := mgm.Coll(&shared.TxData{}).CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func DBGetLatestTxByShardID(shardID int, limit int64) ([]shared.TxData, error) {
	var result []shared.TxData
	filter := bson.M{"shardID": bson.M{operator.Eq: shardID}}
	if shardID == -1 {
		filter = bson.M{}
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(10)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"locktime", -1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBGetTxByMetaAndOTA(pubkey string, metatype int, limit int64, offset int64) ([]shared.TxData, error) {
	var result []shared.TxData
	filter := bson.M{"pubkeyreceivers": bson.M{operator.Eq: pubkey}, "metatype": bson.M{operator.Eq: strconv.Itoa(metatype)}}
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(context.Background(), &result, filter, &options.FindOptions{
		Sort:  bson.D{{"locktime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBUpdateTxsWithPubkeyReceiver(list []shared.TxData) error {
	startTime := time.Now()
	docs := []interface{}{}
	for _, tx := range list {
		update := bson.M{
			"$addToSet": bson.M{"pubkeyreceivers": bson.M{operator.Each: tx.PubKeyReceivers}},
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		_, err := mgm.Coll(&shared.TxData{}).UpdateByID(context.Background(), list[idx].GetID(), doc)
		if err != nil {
			log.Printf("failed to update %v txs in %v", len(list), time.Since(startTime))
			return err
		}
	}
	log.Printf("updated %v txs in %v", len(list), time.Since(startTime))
	return nil
}

func DBGetLatestTradeTx(isswap bool) ([]shared.TradeOrderData, error) {
	limit := int64(20)
	currentTime := time.Now().Unix()
	var result []shared.TradeOrderData
	filter := bson.M{"requesttime": bson.M{operator.Lte: currentTime}, "isswap": bson.M{operator.Eq: isswap}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFind(&result, filter, &options.FindOptions{
		Sort:  bson.D{{"locktime", -1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
