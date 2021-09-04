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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBSavePDEState(state string) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(10)*shared.DB_OPERATION_TIMEOUT)
	upsert := true
	opt := options.FindOneAndUpdateOptions{
		Upsert: &upsert,
	}
	var doc interface{}
	newState := shared.NewPDEStateData(state)
	doc = newState
	err := mgm.Coll(&shared.PDEStateData{}).FindOneAndUpdate(ctx, bson.M{}, doc, &opt)
	if err.Err() != nil {
		log.Println(err)
		return err.Err()
	}
	return nil
}

func DBGetPDEState() (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(4)*shared.DB_OPERATION_TIMEOUT)
	var result shared.PDEStateData
	err := mgm.Coll(&shared.PDEStateData{}).FirstWithCtx(ctx, bson.M{}, &result)
	if err != nil {
		log.Println(err)
		return "", err
	}
	return result.State, nil
}

func DBSaveTxTrade(list []shared.TradeOrderData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.TradeOrderData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.TradeOrderData{}).InsertOne(ctx, v)
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

func DBGetTxTradeFromTxRespond(respondList []string) ([]shared.TradeOrderData, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(respondList)+10)*shared.DB_OPERATION_TIMEOUT)
	result := []shared.TradeOrderData{}

	filter := bson.M{"respondtxs": bson.M{operator.In: respondList}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFindWithCtx(ctx, &result, filter)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return result, nil
}

func DBGetTxTradeFromTxRequest(requestList []string) ([]shared.TradeOrderData, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(requestList)+10)*shared.DB_OPERATION_TIMEOUT)
	result := []shared.TradeOrderData{}

	filter := bson.M{"requesttx": bson.M{operator.In: requestList}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFindWithCtx(ctx, &result, filter)
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

func DBSavePDEContribute(list []shared.ContributionData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.ContributionData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.ContributionData{}).InsertOne(ctx, v)
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

func DBGetPDEContributeRespond(address []string, limit int64, offset int64) ([]shared.ContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.ContributionData
	filter := bson.M{"contributor": bson.M{operator.In: address}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.ContributionData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"respondblock", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBGetPDEV3ContributeRespond(nftID, poolID string, limit int64, offset int64) ([]shared.ContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.ContributionData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftID}, "poolid": bson.M{operator.Eq: poolID}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.ContributionData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBSavePDEWithdraw(list []shared.WithdrawContributionData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.WithdrawContributionData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.WithdrawContributionData{}).InsertOne(ctx, v)
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

func DBGetPDEWithdrawRespond(address []string, limit int64, offset int64) ([]shared.WithdrawContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.WithdrawContributionData
	filter := bson.M{"contributor": bson.M{operator.In: address}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.WithdrawContributionData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPDEV3WithdrawRespond(nftID, poolID string, limit int64, offset int64) ([]shared.WithdrawContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.WithdrawContributionData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftID}, "poolid": bson.M{operator.Eq: poolID}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.WithdrawContributionData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBSavePDEWithdrawFee(list []shared.WithdrawContributionFeeData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.WithdrawContributionFeeData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.WithdrawContributionFeeData{}).InsertOne(ctx, v)
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

func DBGetPDEWithdrawFeeRespond(address []string, limit int64, offset int64) ([]shared.WithdrawContributionFeeData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.WithdrawContributionFeeData
	filter := bson.M{"contributor": bson.M{operator.In: address}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.WithdrawContributionFeeData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBGetPDEV3WithdrawFeeRespond(nftID, poolID string, limit int64, offset int64) ([]shared.WithdrawContributionFeeData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.WithdrawContributionFeeData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftID}, "poolid": bson.M{operator.Eq: poolID}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.WithdrawContributionFeeData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBFindPair(prefix string) ([]shared.PoolPairData, error) {
	var result []shared.PoolPairData

	return result, nil
}

func DBSaveTradeOrder(orders []shared.TradeOrderData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(orders)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range orders {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.TradeOrderData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
			for idx, v := range docs {
				ctx, _ := context.WithTimeout(context.Background(), time.Duration(2)*shared.DB_OPERATION_TIMEOUT)
				filter := bson.M{"requesttx": bson.M{operator.Eq: orders[idx].RequestTx}}
				_, err = mgm.Coll(&shared.TradeOrderData{}).UpdateOne(ctx, filter, v, mgm.UpsertTrueOption())
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

func DBUpdateTradeOrder(orders []shared.TradeOrderData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(orders)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range orders {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$addToSet": bson.M{"respondtxs": order.RespondTxs},
			"$set":      bson.M{"status": order.Status},
		}
		err := mgm.Coll(&shared.TradeOrderData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdateCancelTradeOrder(orders []shared.TradeOrderData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(orders)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range orders {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$addToSet": bson.M{"canceltx": order.CancelTx},
			"$set":      bson.M{"status": order.Status},
		}
		err := mgm.Coll(&shared.TradeOrderData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBGetPendingOrder(pubkey, pairID string) ([]shared.TradeOrderData, error) {
	var result []shared.TradeOrderData

	return result, nil
}

func DBSavePoolPairs(pools []shared.PoolPairData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(pools)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range pools {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.PoolPairData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
			for idx, v := range docs {
				ctx, _ := context.WithTimeout(context.Background(), time.Duration(2)*shared.DB_OPERATION_TIMEOUT)
				filter := bson.M{"txhash": bson.M{operator.Eq: pools[idx].PoolID}}
				_, err = mgm.Coll(&shared.PoolPairData{}).UpdateOne(ctx, filter, v, mgm.UpsertTrueOption())
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

func DBGetPoolPairsByPairID(pairID string) ([]shared.PoolPairData, error) {
	var result []shared.PoolPairData
	filter := bson.M{"pairid": bson.M{operator.Eq: pairID}}
	if pairID == "all" {
		filter = bson.M{}
	}
	err := mgm.Coll(&shared.PoolPairData{}).SimpleFind(result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPoolPairsByPoolID(poolID []string) ([]shared.PoolPairData, error) {
	var result []shared.PoolPairData
	filter := bson.M{"poolid": bson.M{operator.In: poolID}}
	err := mgm.Coll(&shared.PoolPairData{}).SimpleFind(result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPdexPairs() ([]shared.PairData, error) {
	var result []shared.PairData
	err := mgm.Coll(&shared.PairData{}).SimpleFind(result, bson.M{})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// func DBUpdateOrdersOwner(orders []shared.TradeOrderData) error {
// 	startTime := time.Now()
// 	docs := []interface{}{}
// 	for _, order := range orders {
// 		update := bson.M{
// 			"$set": bson.M{"otasecret": order.OTAsecret},
// 		}
// 		docs = append(docs, update)
// 	}
// 	for idx, doc := range docs {
// 		_, err := mgm.Coll(&shared.CoinData{}).UpdateByID(context.Background(), orders[idx].GetID(), doc)
// 		if err != nil {
// 			log.Printf("failed to update %v orders in %v", len(orders), time.Since(startTime))
// 			return err
// 		}
// 	}
// 	log.Printf("updated %v orders in %v", len(orders), time.Since(startTime))
// 	return nil
// }

func DBGetUnknownOrdersFromDB(shardID int, height uint64, limit int64) ([]shared.TradeOrderData, error) {
	var result []shared.TradeOrderData
	filter := bson.M{"nftid": bson.M{operator.Ne: ""}, "otasecret": bson.M{operator.Eq: ""}, "shardid": bson.M{operator.Eq: shardID}, "blockheight": bson.M{operator.Gte: height}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFind(result, filter, &options.FindOptions{
		Sort:  bson.D{{"locktime", 1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBUpdateOrderProgress(orders []shared.LimitOrderStatus) error {
	startTime := time.Now()
	docs := []interface{}{}
	for _, order := range orders {
		update := bson.M{
			"$set": bson.M{"status": order.Status, "left": order.Left, "requesttx": order.RequestTx},
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		_, err := mgm.Coll(&shared.LimitOrderStatus{}).UpdateByID(context.Background(), orders[idx].GetID(), doc)
		if err != nil {
			log.Printf("failed to update %v orders in %v", len(orders), time.Since(startTime))
			return err
		}
	}
	log.Printf("updated %v orders in %v", len(orders), time.Since(startTime))
	return nil
}

func DBUpdatePoolShare(shareList []shared.PoolShareData) error {
	return nil
}

func DBUpdatePoolData(poolList []shared.PoolPairData) error {
	return nil
}

func DBUpdatePDEContribute(list []shared.ContributionData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$addToSet": bson.M{"respondtxs": order.RespondTxs, "returntoken": order.ReturnToken, "returnamount": order.ReturnAmount},
			"$set":      bson.M{},
		}
		err := mgm.Coll(&shared.ContributionData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEWithdraw(list []shared.WithdrawContributionData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$addToSet": bson.M{"respondtxs": order.RespondTxs, "withdrawtoken": order.WithdrawToken, "withdrawamount": order.WithdrawAmount},
			"$set":      bson.M{"status": order.Status},
		}
		err := mgm.Coll(&shared.WithdrawContributionData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEWithdrawFee(list []shared.WithdrawContributionFeeData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$addToSet": bson.M{"respondtxs": order.RespondTxs, "withdrawtoken": order.ReturnTokens, "withdrawamount": order.ReturnAmount},
			"$set":      bson.M{"status": order.Status},
		}
		err := mgm.Coll(&shared.WithdrawContributionFeeData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEPoolPairData(list []shared.PoolPairData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, pool := range list {
		fitler := bson.M{"poolid": bson.M{operator.Eq: pool.PoolID}}
		update := bson.M{
			"$set": bson.M{"pairid": pool.PairID, "tokenid1": pool.TokenID1, "tokenid2": pool.TokenID2, "token1amount": pool.Token1Amount, "token2amount": pool.Token2Amount, "amp": pool.AMP, "version": pool.Version},
		}
		err := mgm.Coll(&shared.WithdrawContributionFeeData{}).FindOneAndUpdate(ctx, fitler, update, options.FindOneAndUpdate().SetUpsert(true))
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEPoolShareData(list []shared.PoolShareData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, share := range list {
		fitler := bson.M{"nftid": bson.M{operator.Eq: share.NFTID}}
		update := bson.M{
			"$set": bson.M{"poolid": share.PoolID, "amount": share.Amount, "tradingfee": share.TradingFee},
		}
		err := mgm.Coll(&shared.WithdrawContributionFeeData{}).FindOneAndUpdate(ctx, fitler, update, options.FindOneAndUpdate().SetUpsert(true))
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEPoolStakeData(list []shared.PoolStakeData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, stake := range list {
		fitler := bson.M{"tokenid": bson.M{operator.Eq: stake.TokenID}}
		update := bson.M{
			"$set": bson.M{"tokenid": stake.TokenID, "amount": stake.Amount},
		}
		err := mgm.Coll(&shared.PoolStakeData{}).FindOneAndUpdate(ctx, fitler, update, options.FindOneAndUpdate().SetUpsert(true))
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEPoolStakerData(list []shared.PoolStakerData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, stake := range list {
		fitler := bson.M{"nftid": bson.M{operator.Eq: stake.NFTID}}
		update := bson.M{
			"$set": bson.M{"nftid": stake.NFTID, "amount": stake.Amount, "reward": stake.Reward},
		}
		err := mgm.Coll(&shared.PoolStakerData{}).FindOneAndUpdate(ctx, fitler, update, options.FindOneAndUpdate().SetUpsert(true))
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBGetTradeInfoAndStatus(requestTx string) (*shared.TradeOrderData, *shared.LimitOrderStatus, error) {
	var tradeInfo shared.TradeOrderData
	var tradeStatus shared.LimitOrderStatus
	filter := bson.M{"requesttx": bson.M{operator.Eq: requestTx}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFind(&tradeInfo, filter)
	if err != nil {
		return nil, nil, err
	}
	err = mgm.Coll(&shared.LimitOrderStatus{}).SimpleFind(&tradeStatus, filter)
	if err != nil {
		return &tradeInfo, nil, err
	}
	return &tradeInfo, &tradeStatus, nil
}

func DBGetTradeStatus(requestTx []string) (map[string]shared.LimitOrderStatus, error) {
	var tradeStatus []shared.LimitOrderStatus
	filter := bson.M{"requesttx": bson.M{operator.In: requestTx}}
	err := mgm.Coll(&shared.LimitOrderStatus{}).SimpleFind(&tradeStatus, filter)
	if err != nil {
		return nil, err
	}
	result := make(map[string]shared.LimitOrderStatus)
	for _, v := range tradeStatus {
		result[v.RequestTx] = v
	}
	return result, nil
}

func DBGetShare(nftID string) ([]shared.PoolShareData, error) {
	var result []shared.PoolShareData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftID}}
	err := mgm.Coll(&shared.PoolShareData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetTxTradeFromPoolAndNFT(poolid, nftid string, limit, offset int64) ([]shared.TradeOrderData, error) {
	var result []shared.TradeOrderData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftid}, "poolid": bson.M{operator.Eq: poolid}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFind(&result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
