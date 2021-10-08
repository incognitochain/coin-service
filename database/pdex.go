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

func DBSavePDEState(state string, version int) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(10)*shared.DB_OPERATION_TIMEOUT)
	var doc interface{}
	newState := shared.NewPDEStateData(state, version)
	doc = newState
	filter := bson.M{"version": bson.M{operator.Eq: version}}
	_, err := mgm.Coll(&shared.PDEStateData{}).UpdateOne(ctx, filter, doc, mgm.UpsertTrueOption())
	if err != nil {
		return err
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
	if len(list) == 0 {
		return nil
	}
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
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"nftid": bson.M{operator.Eq: order.NFTID}, "pairhash": bson.M{operator.Eq: order.PairHash}, "requesttxs.1": bson.M{operator.Exists: false}}
		update := bson.M{
			"$push": bson.M{"requesttxs": bson.M{operator.Each: order.RequestTxs}, "contributetokens": bson.M{operator.Each: order.ContributeTokens}, "contributeamount": bson.M{operator.Each: order.ContributeAmount}},
			"$set":  bson.M{"pairhash": order.PairHash, "nftid": order.NFTID, "poolid": order.PoolID, "requesttime": order.RequestTime},
		}
		_, err := mgm.Coll(&shared.ContributionData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
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
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBGetPDEV3ContributeRespond(nftID string, limit int64, offset int64) ([]shared.ContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.ContributionData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftID}}
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

func DBGetPDEV3ContributeWaiting(nftID string, limit int64, offset int64) ([]shared.ContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.ContributionData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftID}, "status": bson.M{operator.Eq: "waiting"}}
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
	if len(list) == 0 {
		return nil
	}
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
	filter := bson.M{}
	if poolID != "" {
		filter = bson.M{"nftid": bson.M{operator.Eq: nftID}, "poolid": bson.M{operator.Eq: poolID}}
	} else {
		filter = bson.M{"nftid": bson.M{operator.Eq: nftID}}
	}
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
	if len(list) == 0 {
		return nil
	}
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
	filter := bson.M{}
	if poolID != "" {
		filter = bson.M{"nftid": bson.M{operator.Eq: nftID}, "poolid": bson.M{operator.Eq: poolID}}
	} else {
		filter = bson.M{"nftid": bson.M{operator.Eq: nftID}}
	}
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
	//TODO
	return result, nil
}

func DBSaveTradeOrder(orders []shared.TradeOrderData) error {
	if len(orders) == 0 {
		return nil
	}
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
	for _, order := range orders {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(1*shared.DB_OPERATION_TIMEOUT))
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$push": bson.M{"respondtxs": bson.M{operator.Each: order.RespondTxs}, "respondtokens": bson.M{operator.Each: order.RespondTokens}, "respondamount": bson.M{operator.Each: order.RespondAmount}},
			"$set":  bson.M{"status": order.Status},
		}
		_, err := mgm.Coll(&shared.TradeOrderData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdateWithdrawTradeOrderReq(orders []shared.TradeOrderData) error {
	if len(orders) == 0 {
		return nil
	}
	for _, order := range orders {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(1*shared.DB_OPERATION_TIMEOUT))
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$push": bson.M{"withdrawtxs": bson.M{operator.Each: order.WithdrawTxs}},
			"$set":  bson.M{"withdrawinfos." + order.WithdrawTxs[0]: order.WithdrawInfos[order.WithdrawTxs[0]]},
		}
		_, err := mgm.Coll(&shared.TradeOrderData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdateWithdrawTradeOrderRes(orders []shared.TradeOrderData) error {
	if len(orders) == 0 {
		return nil
	}
	for _, order := range orders {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(1*shared.DB_OPERATION_TIMEOUT))
		fitler := bson.M{"withdrawtxs": bson.M{operator.Eq: order.WithdrawTxs[0]}}
		prefix := "withdrawinfos." + order.WithdrawTxs[0]
		update := bson.M{
			"$push": bson.M{prefix + ".responds": bson.M{operator.Each: order.WithdrawInfos[order.WithdrawTxs[0]].Responds}, prefix + ".status": bson.M{operator.Each: order.WithdrawInfos[order.WithdrawTxs[0]].Status}, prefix + ".respondtokens": bson.M{operator.Each: order.WithdrawInfos[order.WithdrawTxs[0]].RespondTokens}, prefix + ".respondamount": bson.M{operator.Each: order.WithdrawInfos[order.WithdrawTxs[0]].RespondAmount}},
		}
		_, err := mgm.Coll(&shared.TradeOrderData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

// func DBGetPendingOrder(pubkey, pairID string) ([]shared.TradeOrderData, error) {
// 	var result []shared.TradeOrderData
// 	//TODO
// 	return result, nil
// }

func DBSavePoolPairs(pools []shared.PoolPairData) error {
	if len(pools) == 0 {
		return nil
	}
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
	filter := bson.M{"pairid": bson.M{operator.Eq: pairID}, "version": bson.M{operator.Eq: 2}}
	if pairID == "all" {
		filter = bson.M{}
	}
	err := mgm.Coll(&shared.PoolPairData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPoolPairsByPoolID(poolID []string) ([]shared.PoolPairData, error) {
	var result []shared.PoolPairData
	filter := bson.M{"poolid": bson.M{operator.In: poolID}}
	err := mgm.Coll(&shared.PoolPairData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPdexPairs() ([]shared.PairData, error) {
	var result []shared.PairData
	err := mgm.Coll(&shared.PairData{}).SimpleFind(&result, bson.M{})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBUpdatePDEContributeRespond(list []shared.ContributionData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttxs": bson.M{operator.In: order.RequestTxs}}
		update := bson.M{
			"$push": bson.M{"respondtxs": bson.M{operator.Each: order.RespondTxs}, "returntokens": bson.M{operator.Each: order.ReturnTokens}, "returnamount": bson.M{operator.Each: order.ReturnAmount}},
		}
		_, err := mgm.Coll(&shared.ContributionData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdatePDEWithdraw(list []shared.WithdrawContributionData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$push": bson.M{"respondtxs": bson.M{operator.Each: order.RespondTxs}, "withdrawtokens": bson.M{operator.Each: order.WithdrawTokens}, "withdrawamount": bson.M{operator.Each: order.WithdrawAmount}},
			"$set":  bson.M{"status": order.Status},
		}
		_, err := mgm.Coll(&shared.WithdrawContributionData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdatePDEWithdrawFee(list []shared.WithdrawContributionFeeData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$push": bson.M{"respondtxs": bson.M{operator.Each: order.RespondTxs}, "withdrawtokens": bson.M{operator.Each: order.WithdrawTokens}, "withdrawamount": bson.M{operator.Each: order.WithdrawAmount}},
			"$set":  bson.M{"status": order.Status},
		}
		_, err := mgm.Coll(&shared.WithdrawContributionFeeData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdatePDEPairListData(list []shared.PairData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, pool := range list {
		fitler := bson.M{"PairData": bson.M{operator.Eq: pool.PairID}}
		update := bson.M{
			"$set": bson.M{"pairid": pool.PairID, "tokenid1": pool.TokenID1, "tokenid2": pool.TokenID2, "token1amount": pool.Token1Amount, "token2amount": pool.Token2Amount},
		}
		_, err := mgm.Coll(&shared.PairData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdatePDEPoolPairData(list []shared.PoolPairData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, pool := range list {
		fitler := bson.M{"poolid": bson.M{operator.Eq: pool.PoolID}}
		update := bson.M{
			"$set": bson.M{"pairid": pool.PairID, "tokenid1": pool.TokenID1, "tokenid2": pool.TokenID2, "token1amount": pool.Token1Amount, "token2amount": pool.Token2Amount, "virtual1amount": pool.Virtual1Amount, "virtual2amount": pool.Virtual2Amount, "amp": pool.AMP, "version": pool.Version, "totalshare": pool.TotalShare},
		}
		_, err := mgm.Coll(&shared.PoolPairData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdatePDEPoolShareData(list []shared.PoolShareData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, share := range list {
		fitler := bson.M{"nftid": bson.M{operator.Eq: share.NFTID}, "poolid": bson.M{operator.Eq: share.PoolID}}
		update := bson.M{
			"$set": bson.M{"poolid": share.PoolID, "amount": share.Amount, "tradingfee": share.TradingFee},
		}
		_, err := mgm.Coll(&shared.PoolShareData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdateOrderProgress(list []shared.LimitOrderStatus) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$set": bson.M{"token1balance": order.Token1Balance, "token2balance": order.Token2Balance, "direction": order.Direction},
		}
		_, err := mgm.Coll(&shared.LimitOrderStatus{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdatePDEPoolStakeData(list []shared.PoolStakeData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, stake := range list {
		fitler := bson.M{"tokenid": bson.M{operator.Eq: stake.TokenID}}
		update := bson.M{
			"$set": bson.M{"tokenid": stake.TokenID, "amount": stake.Amount},
		}
		_, err := mgm.Coll(&shared.PoolStakeData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdatePDEPoolStakerData(list []shared.PoolStakerData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, stake := range list {
		fitler := bson.M{"nftid": bson.M{operator.Eq: stake.NFTID}, "tokenid": bson.M{operator.Eq: stake.TokenID}}
		update := bson.M{
			"$set": bson.M{"nftid": stake.NFTID, "tokenid": stake.TokenID, "amount": stake.Amount, "reward": stake.Reward},
		}
		_, err := mgm.Coll(&shared.PoolStakerData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func DBGetTradeInfoAndStatus(requestTx string) (*shared.TradeOrderData, *shared.LimitOrderStatus, error) {
	var tradeInfo []shared.TradeOrderData
	var tradeStatus []shared.LimitOrderStatus
	filter := bson.M{"requesttx": bson.M{operator.Eq: requestTx}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFind(&tradeInfo, filter)
	if err != nil {
		return nil, nil, err
	}
	err = mgm.Coll(&shared.LimitOrderStatus{}).SimpleFind(&tradeStatus, filter)
	if err != nil {
		return &tradeInfo[0], nil, err
	}
	if len(tradeStatus) == 0 {
		return &tradeInfo[0], nil, nil
	}
	return &tradeInfo[0], &tradeStatus[0], nil
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

func DBGetStakePools() ([]shared.PoolStakeData, error) {
	var result []shared.PoolStakeData
	filter := bson.M{}
	err := mgm.Coll(&shared.PoolStakeData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetStakingPoolHistory(nftid, tokenid string, limit, offset int64) ([]shared.PoolStakeHistoryData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.PoolStakeHistoryData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftid}, "tokenid": bson.M{operator.Eq: tokenid}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.PoolStakeHistoryData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBGetStakePoolRewardHistory(nftid, tokenid string, limit, offset int64) ([]shared.PoolStakeRewardHistoryData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.PoolStakeRewardHistoryData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftid}, "tokenid": bson.M{operator.Eq: tokenid}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.PoolStakeRewardHistoryData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBGetStakingInfo(nftid string) ([]shared.PoolStakerData, error) {
	var result []shared.PoolStakerData
	filter := bson.M{"nftid": bson.M{operator.Eq: nftid}}
	err := mgm.Coll(&shared.PoolStakerData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBSavePDEStakeHistory(list []shared.PoolStakeHistoryData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.PoolStakeHistoryData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.PoolStakeHistoryData{}).InsertOne(ctx, v)
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

func DBSavePDEStakeRewardHistory(list []shared.PoolStakeRewardHistoryData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.PoolStakeRewardHistoryData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.PoolStakeRewardHistoryData{}).InsertOne(ctx, v)
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

func DBUpdatePDEStakingHistory(list []shared.PoolStakeHistoryData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$set": bson.M{"status": order.Status, "respondtx": order.RespondTx},
		}
		err := mgm.Coll(&shared.PoolStakeHistoryData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEStakeRewardHistory(list []shared.PoolStakeRewardHistoryData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$set": bson.M{"status": order.Status, "respondtx": order.RespondTx, "amount": order.Amount},
		}
		err := mgm.Coll(&shared.PoolStakeRewardHistoryData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBDeletePDEPoolData(list []shared.PoolPairData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, pool := range list {
		fitler := bson.M{"poolid": bson.M{operator.Eq: pool.PoolID}}
		err := mgm.Coll(&shared.PoolPairData{}).FindOneAndDelete(ctx, fitler)
		if err != nil {
			if err.Err() != mongo.ErrNoDocuments {
				return err.Err()
			}
		}
	}
	return nil
}

func DBDeletePDEPoolShareData(list []shared.PoolShareData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, share := range list {
		fitler := bson.M{"nftid": bson.M{operator.Eq: share.NFTID}, "poolid": bson.M{operator.Eq: share.PoolID}}
		err := mgm.Coll(&shared.PoolShareData{}).FindOneAndDelete(ctx, fitler)
		if err != nil {
			if err.Err() != mongo.ErrNoDocuments {
				return err.Err()
			}
		}
	}
	return nil
}

func DBDeletePDEPoolStakeData(list []shared.PoolStakeData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, stake := range list {
		fitler := bson.M{"tokenid": bson.M{operator.Eq: stake.TokenID}}
		err := mgm.Coll(&shared.PoolStakeData{}).FindOneAndDelete(ctx, fitler)
		if err != nil {
			if err.Err() != mongo.ErrNoDocuments {
				return err.Err()
			}
		}
	}
	return nil
}

func DBDeletePDEPoolStakerData(list []shared.PoolStakerData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, stake := range list {
		fitler := bson.M{"nftid": bson.M{operator.Eq: stake.NFTID}, "tokenid": bson.M{operator.Eq: stake.TokenID}}
		err := mgm.Coll(&shared.PoolStakerData{}).FindOneAndDelete(ctx, fitler)
		if err != nil {
			if err.Err() != mongo.ErrNoDocuments {
				return err.Err()
			}
		}
	}
	return nil
}

func DBDeleteOrderProgress(list []shared.LimitOrderStatus) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, v := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: v.RequestTx}}
		err := mgm.Coll(&shared.LimitOrderStatus{}).FindOneAndDelete(ctx, fitler)
		if err != nil {
			if err.Err() != mongo.ErrNoDocuments {
				return err.Err()
			}
		}
	}
	return nil
}

func DBSaveInstructionBeacon(list []shared.InstructionBeaconData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.InstructionBeaconData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.InstructionBeaconData{}).InsertOne(ctx, v)
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

func DBGetPendingOrder(limit int64, offset int64) ([]shared.TradeOrderData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.TradeOrderData
	filter := bson.M{"status": bson.M{operator.Eq: 0}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPendingLiquidityWithdraw(limit int64, offset int64) ([]shared.WithdrawContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.WithdrawContributionData
	filter := bson.M{"status": bson.M{operator.Eq: 0}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.WithdrawContributionData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPendingLiquidityWithdrawFee(limit int64, offset int64) ([]shared.WithdrawContributionFeeData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.WithdrawContributionFeeData
	filter := bson.M{"status": bson.M{operator.Eq: 0}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.WithdrawContributionFeeData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPendingUnstakingPool(limit int64, offset int64) ([]shared.PoolStakeHistoryData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.PoolStakeHistoryData
	filter := bson.M{"status": bson.M{operator.Eq: 0}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.PoolStakeHistoryData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPendingWithdrawRewardStakingPool(limit int64, offset int64) ([]shared.PoolStakeRewardHistoryData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.PoolStakeRewardHistoryData
	filter := bson.M{"status": bson.M{operator.Eq: 0}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.PoolStakeRewardHistoryData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBUpdatePDELiquidityWithdrawStatus(list []shared.WithdrawContributionData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$set": bson.M{"status": order.Status},
		}
		err := mgm.Coll(&shared.WithdrawContributionData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDELiquidityWithdrawFeeStatus(list []shared.WithdrawContributionFeeData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$set": bson.M{"status": order.Status},
		}
		err := mgm.Coll(&shared.WithdrawContributionFeeData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEUnstakingPoolStatus(list []shared.PoolStakeHistoryData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$set": bson.M{"status": order.Status},
		}
		err := mgm.Coll(&shared.PoolStakeHistoryData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBUpdatePDEWithdrawRewardStakingStatus(list []shared.PoolStakeRewardHistoryData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$set": bson.M{"status": order.Status},
		}
		err := mgm.Coll(&shared.PoolStakeRewardHistoryData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBGetBeaconInstructionByTx(txhash string) (*shared.InstructionBeaconData, error) {
	var result shared.InstructionBeaconData
	filter := bson.M{"txrequest": bson.M{operator.Eq: txhash}}
	ctx, _ := context.WithTimeout(context.Background(), 2*shared.DB_OPERATION_TIMEOUT)
	data := mgm.Coll(&shared.InstructionBeaconData{}).FindOne(ctx, filter)
	if data.Err() != nil {
		if data.Err() == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, data.Err()
	}
	data.Decode(&result)
	return &result, nil
}

func DBGetPendingWithdrawOrder(limit int64, offset int64) ([]shared.TradeOrderData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.TradeOrderData
	filter := bson.M{"withdrawinfos.$.responds": bson.M{operator.Eq: nil}, "withdrawtxs": bson.M{operator.Not: bson.M{operator.Size: 0}}, "withdrawinfos.$.isrejected": bson.M{operator.Eq: false}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBUpdatePDETradeWithdrawStatus(list []shared.TradeOrderData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: order.RequestTx}}
		update := bson.M{
			"$set": bson.M{"withdrawinfos": order.WithdrawInfos},
		}
		err := mgm.Coll(&shared.TradeOrderData{}).FindOneAndUpdate(ctx, fitler, update)
		if err != nil {
			return err.Err()
		}
	}
	return nil
}

func DBGetPairsByID(list []string) ([]shared.PairData, error) {
	var result []shared.PairData
	filter := bson.M{"pairid": bson.M{operator.In: list}}
	err := mgm.Coll(&shared.PairData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}
