package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBSavePDEState(state string, height uint64, version int) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(10)*shared.DB_OPERATION_TIMEOUT)

	doc := bson.M{
		"$set": bson.M{
			"version": version,
			"state":   state,
			"height":  height,
		},
	}
	filter := bson.M{"version": bson.M{operator.Eq: version}}
	_, err := mgm.Coll(&shared.PDEStateData{}).UpdateOne(ctx, filter, doc, mgm.UpsertTrueOption())
	if err != nil {
		return err
	}
	return nil
}

func DBGetPDEState(version int) (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(4)*shared.DB_OPERATION_TIMEOUT)
	var result shared.PDEStateData
	err := mgm.Coll(&shared.PDEStateData{}).FirstWithCtx(ctx, bson.M{"version": bson.M{operator.Eq: version}}, &result)
	if err != nil {
		log.Println(err)
		return "", err
	}
	return result.State, nil
}

func DBGetPDEStateWithHeight(version int, height uint64) (*shared.PDEStateData, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(4)*shared.DB_OPERATION_TIMEOUT)
	var result shared.PDEStateData
	err := mgm.Coll(&shared.PDEStateData{}).FirstWithCtx(ctx, bson.M{"version": bson.M{operator.Eq: version}, "height": bson.M{operator.Gt: height}}, &result)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &result, nil
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
	filter := bson.M{"pubkeyreceivers": bson.M{operator.In: []string{pubkey}}, "metatype": bson.M{operator.In: metas}}
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
		if order.Version == 3 {
			fitler := bson.M{"nftid": bson.M{operator.Eq: order.NFTID}, "pairhash": bson.M{operator.Eq: order.PairHash}, "requesttxs.1": bson.M{operator.Exists: false}}
			update := bson.M{
				"$push": bson.M{"requesttxs": bson.M{operator.Each: order.RequestTxs}, "contributetokens": bson.M{operator.Each: order.ContributeTokens}, "contributeamount": bson.M{operator.Each: order.ContributeAmount}},
				"$set":  bson.M{"pairhash": order.PairHash, "nftid": order.NFTID, "poolid": order.PoolID, "requesttime": order.RequestTime, "contributor": order.Contributor, "pairid": order.PairID},
			}
			_, err := mgm.Coll(&shared.ContributionData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
			if err != nil {
				return err
			}
		} else {
			fitler := bson.M{"pairid": bson.M{operator.Eq: order.PairID}, "requesttxs.1": bson.M{operator.Exists: false}}
			update := bson.M{
				"$push": bson.M{"requesttxs": bson.M{operator.Each: order.RequestTxs}, "contributetokens": bson.M{operator.Each: order.ContributeTokens}, "contributeamount": bson.M{operator.Each: order.ContributeAmount}},
				"$set":  bson.M{"requesttime": order.RequestTime, "contributor": order.Contributor, "pairid": order.PairID},
			}
			_, err := mgm.Coll(&shared.ContributionData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func DBGetPDEContributeRespond(address string, limit int64, offset int64) ([]shared.ContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.ContributionData
	filter := bson.M{"contributor": bson.M{operator.Eq: address}}
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

func DBGetPDEV3ContributeRespond(nftID []string, limit int64, offset int64) ([]shared.ContributionData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.ContributionData
	filter := bson.M{"nftid": bson.M{operator.In: nftID}}
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

// func DBGetPDEWithdrawRespond(address []string, limit int64, offset int64) ([]shared.WithdrawContributionData, error) {
// 	if limit == 0 {
// 		limit = int64(10000)
// 	}
// 	var result []shared.WithdrawContributionData
// 	filter := bson.M{"contributor": bson.M{operator.In: address}}
// 	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
// 	err := mgm.Coll(&shared.WithdrawContributionData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
// 		Sort:  bson.D{{"requesttime", -1}},
// 		Skip:  &offset,
// 		Limit: &limit,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return result, nil
// }

func DBGetPDEWithdrawByRespondTx(txlist []string) ([]shared.WithdrawContributionData, error) {
	var result []shared.WithdrawContributionData
	filter := bson.M{"respondtxs": bson.M{operator.In: txlist}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(txlist))*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.WithdrawContributionData{}).SimpleFindWithCtx(ctx, &result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}
func DBGetPDEWithdrawFeeByRespondTx(txlist []string) ([]shared.WithdrawContributionFeeData, error) {
	var result []shared.WithdrawContributionFeeData
	filter := bson.M{"respondtxs": bson.M{operator.In: txlist}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(txlist))*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.WithdrawContributionFeeData{}).SimpleFindWithCtx(ctx, &result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPDETXWithdrawRespond(pubkey []string, limit int64, offset int64) ([]shared.TxData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.TxData
	filter := bson.M{"pubkeyreceivers": bson.M{operator.In: pubkey}, "metatype": bson.M{operator.Eq: strconv.Itoa(metadataCommon.PDEWithdrawalResponseMeta)}}
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

func DBGetPDETXWithdrawFeeRespond(pubkey []string, limit int64, offset int64) ([]shared.TxData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.TxData
	filter := bson.M{"pubkeyreceivers": bson.M{operator.In: pubkey}, "metatype": bson.M{operator.Eq: strconv.Itoa(metadataCommon.PDEFeeWithdrawalResponseMeta)}}
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

func DBFindPair(prefix string) ([]shared.PoolInfoData, error) {
	var result []shared.PoolInfoData
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
			d, _ := json.Marshal(docs)
			fmt.Println(string(d))
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
			for idx, _ := range docs {
				ctx, _ := context.WithTimeout(context.Background(), time.Duration(2)*shared.DB_OPERATION_TIMEOUT)
				od := orders[idx]
				filter := bson.M{"requesttx": bson.M{operator.Eq: od.RequestTx}}
				update := bson.M{
					"$set": bson.M{"amount": od.Amount, "minaccept": od.MinAccept, "selltokenid": od.SellTokenID, "buytokenid": od.BuyTokenID, "requesttx": od.RequestTx, "tradingpath": od.TradingPath, "version": od.Version, "isswap": od.IsSwap, "fee": od.Fee, "feetoken": od.FeeToken, "blockheight": od.BlockHeight, "shardid": od.ShardID, "receiver": od.Receiver, "nftid": od.NFTID, "requesttime": od.Requesttime, "poolid": od.PoolID, "pairid": od.PairID},
				}
				_, err = mgm.Coll(&shared.TradeOrderData{}).UpdateOne(ctx, filter, update, mgm.UpsertTrueOption())
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
		_, err := mgm.Coll(&shared.TradeOrderData{}).UpdateOne(ctx, fitler, update, mgm.UpsertTrueOption())
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
			"$push":     bson.M{"withdrawtxs": bson.M{operator.Each: order.WithdrawTxs}},
			"$addToSet": bson.M{"withdrawpendings": bson.M{operator.Each: order.WithdrawPendings}},
			"$set":      bson.M{"withdrawinfos." + order.WithdrawTxs[0]: order.WithdrawInfos[order.WithdrawTxs[0]]},
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
		// ctx, _ := context.WithTimeout(context.Background(), time.Duration(1*shared.DB_OPERATION_TIMEOUT))
		fitler := bson.M{"withdrawtxs": bson.M{operator.Eq: order.WithdrawTxs[0]}}
		prefix := "withdrawinfos." + order.WithdrawTxs[0]
		update := bson.M{
			"$push": bson.M{prefix + ".responds": bson.M{operator.Each: order.WithdrawInfos[order.WithdrawTxs[0]].Responds}, prefix + ".status": bson.M{operator.Each: order.WithdrawInfos[order.WithdrawTxs[0]].Status}, prefix + ".respondtokens": bson.M{operator.Each: order.WithdrawInfos[order.WithdrawTxs[0]].RespondTokens}, prefix + ".respondamount": bson.M{operator.Each: order.WithdrawInfos[order.WithdrawTxs[0]].RespondAmount}},
			"$pull": bson.M{"withdrawpendings": bson.M{operator.In: order.WithdrawPendings}},
		}
		result, err := mgm.Coll(&shared.TradeOrderData{}).UpdateOne(context.Background(), fitler, update)
		if err != nil {
			return err
		}
		// fully matched auto-gen response
		if result.MatchedCount == 0 {
			// ctx, _ := context.WithTimeout(context.Background(), time.Duration(1*shared.DB_OPERATION_TIMEOUT))
			fitler := bson.M{"requesttx": bson.M{operator.Eq: order.WithdrawTxs[0]}}
			update := bson.M{
				"$set": bson.M{
					"withdrawinfos": order.WithdrawInfos,
				},
			}
			_, err := mgm.Coll(&shared.TradeOrderData{}).UpdateOne(context.Background(), fitler, update)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DBSavePoolPairs(pools []shared.PoolInfoData) error {
	if len(pools) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(pools)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range pools {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.PoolInfoData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.PoolInfoData{}).UpdateOne(ctx, filter, v, mgm.UpsertTrueOption())
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

func DBGetPoolPairsByPairID(pairID string) ([]shared.PoolInfoData, error) {
	var result []shared.PoolInfoData
	filter := bson.M{"pairid": bson.M{operator.Eq: pairID}, "version": bson.M{operator.Eq: 2}}
	if pairID == "all" {
		filter = bson.M{"version": bson.M{operator.Eq: 2}}
	}
	err := mgm.Coll(&shared.PoolInfoData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPoolPairsByPoolID(poolID []string) ([]shared.PoolInfoData, error) {
	var result []shared.PoolInfoData
	filter := bson.M{"poolid": bson.M{operator.In: poolID}}
	err := mgm.Coll(&shared.PoolInfoData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBGetPdexPairs() ([]shared.PairInfoData, error) {
	var result []shared.PairInfoData
	err := mgm.Coll(&shared.PairInfoData{}).SimpleFind(&result, bson.M{})
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

func DBUpdatePDEPairListData(list []shared.PairInfoData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, pool := range list {
		fitler := bson.M{"pairid": bson.M{operator.Eq: pool.PairID}}
		update := bson.M{
			"$set": bson.M{"updated_at": time.Now().UTC(), "pairid": pool.PairID, "tokenid1": pool.TokenID1, "tokenid2": pool.TokenID2, "token1amount": pool.Token1Amount, "token2amount": pool.Token2Amount, "poolcount": pool.PoolCount},
		}
		_, err := mgm.Coll(&shared.PairInfoData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
		if err != nil {
			return err
		}
	}
	return nil
}

func DBUpdatePDEPoolInfoData(list []shared.PoolInfoData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, pool := range list {
		fitler := bson.M{"poolid": bson.M{operator.Eq: pool.PoolID}}
		update := bson.M{
			"$set": bson.M{"updated_at": time.Now().UTC(), "pairid": pool.PairID, "tokenid1": pool.TokenID1, "tokenid2": pool.TokenID2, "token1amount": pool.Token1Amount, "token2amount": pool.Token2Amount, "virtual1amount": pool.Virtual1Amount, "virtual2amount": pool.Virtual2Amount, "amp": pool.AMP, "version": pool.Version, "totalshare": pool.TotalShare},
		}
		_, err := mgm.Coll(&shared.PoolInfoData{}).UpdateOne(ctx, fitler, update, options.Update().SetUpsert(true))
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
			"$set": bson.M{"updated_at": time.Now().UTC(), "poolid": share.PoolID, "amount": share.Amount, "tradingfee": share.TradingFee, "version": share.Version, "orderreward": share.OrderReward, "currentaccess": share.CurrentAccessID},
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
			"$set": bson.M{"updated_at": time.Now().UTC(), "pairid": order.PairID, "poolid": order.PoolID, "token1balance": order.Token1Balance, "token2balance": order.Token2Balance, "direction": order.Direction, "nftid": order.NftID},
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
			"$set": bson.M{"updated_at": time.Now().UTC(), "tokenid": stake.TokenID, "amount": stake.Amount},
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
			"$set": bson.M{"updated_at": time.Now().UTC(), "nftid": stake.NFTID, "tokenid": stake.TokenID, "amount": stake.Amount, "reward": stake.Reward},
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

func DBGetShare(nftID []string) ([]shared.PoolShareData, error) {
	var result []shared.PoolShareData
	filter := bson.M{"nftid": bson.M{operator.In: nftID}}
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

func DBGetTxTradeFromPoolAndAccessID(poolid string, accessIDs []string, limit, offset int64) ([]shared.TradeOrderData, error) {
	var result []shared.TradeOrderData
	filter := bson.M{"nftid": bson.M{operator.In: accessIDs}, "poolid": bson.M{operator.Eq: poolid}}
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
			"$set":  bson.M{"status": order.Status},
			"$push": bson.M{"respondtxs": bson.M{operator.Each: order.RespondTxs}, "amount": bson.M{operator.Each: order.Amount}, "rewardtokens": bson.M{operator.Each: order.RewardTokens}},
		}
		_, err := mgm.Coll(&shared.PoolStakeRewardHistoryData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBDeletePDEPoolData(list []shared.PoolInfoData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	poolIDs := []string{}
	for _, pool := range list {
		poolIDs = append(poolIDs, pool.PoolID)
	}
	fitler := bson.M{"poolid": bson.M{operator.In: poolIDs}}
	_, err := mgm.Coll(&shared.PoolInfoData{}).DeleteMany(ctx, fitler)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return err
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
		_, err := mgm.Coll(&shared.PoolShareData{}).DeleteOne(ctx, fitler)
		if err != nil {
			if err != mongo.ErrNoDocuments {
				return err
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
		_, err := mgm.Coll(&shared.PoolStakeData{}).DeleteOne(ctx, fitler)
		if err != nil {
			if err != mongo.ErrNoDocuments {
				return err
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
		_, err := mgm.Coll(&shared.PoolStakerData{}).DeleteOne(ctx, fitler)
		if err != nil {
			if err != mongo.ErrNoDocuments {
				return err
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
		_, err := mgm.Coll(&shared.LimitOrderStatus{}).DeleteOne(ctx, fitler)
		if err != nil {
			if err != mongo.ErrNoDocuments {
				return err
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

func DBSaveRewardRecord(list []shared.RewardRecord) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.RewardRecord{}).InsertMany(ctx, docs)
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
				_, err = mgm.Coll(&shared.RewardRecord{}).InsertOne(ctx, v)
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

func DBGetPendingOrderByPairID(pairID string) ([]shared.TradeOrderData, error) {
	limit := int64(10000)
	var result []shared.TradeOrderData
	emptyMap := make(map[string]shared.TradeOrderWithdrawInfo)
	filter := bson.M{"pairid": bson.M{operator.Eq: pairID}, "status": bson.M{operator.Eq: 0}, "isswap": bson.M{operator.Eq: false}, "withdrawtxs": bson.M{operator.Eq: []string{}}, "withdrawinfos": bson.M{operator.Eq: emptyMap}, "respondtxs": bson.M{operator.Eq: []string{}}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
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
	var results []shared.InstructionBeaconData
	filter := bson.M{"txrequest": bson.M{operator.Eq: txhash}}
	// ctx, _ := context.WithTimeout(context.Background(), 2*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.InstructionBeaconData{}).SimpleFind(&results, filter)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

func DBGetPendingWithdrawOrder(limit int64, offset int64) ([]shared.TradeOrderData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.TradeOrderData
	filter := bson.M{"withdrawpendings": bson.M{operator.Not: bson.M{operator.Size: 0}, operator.Exists: true}}
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
			"$set":  bson.M{"withdrawinfos": order.WithdrawInfos},
			"$pull": bson.M{"withdrawpendings": bson.M{operator.In: order.WithdrawPendings}},
		}
		_, err := mgm.Coll(&shared.TradeOrderData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBGetPendingRequestStakingPool(limit int64, offset int64) ([]shared.PoolStakeHistoryData, error) {
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

func DBUpdateRequestStakingPoolStatus(list []shared.PoolStakeHistoryData) error {
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

func DBGetPairsByID(list []string) ([]shared.PairInfoData, error) {
	var result []shared.PairInfoData
	filter := bson.M{"pairid": bson.M{operator.In: list}}
	err := mgm.Coll(&shared.PairInfoData{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBUpdatePDEPoolPairRewardAPY(list []shared.RewardAPYTracking) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, order := range list {
		fitler := bson.M{"dataid": bson.M{operator.Eq: order.DataID}}
		update := bson.M{
			"$set": bson.M{"dataid": order.DataID, "apy": order.APY, "beaconheight": order.BeaconHeight, "apy2": order.APY2, "totalreceive": order.TotalReceive, "totalamount": order.TotalAmount},
		}
		_, err := mgm.Coll(&shared.RewardAPYTracking{}).UpdateOne(ctx, fitler, update, mgm.UpsertTrueOption())
		if err != nil {
			fmt.Println("DBUpdatePDEPoolPairRewardAPY", order)
			return err
		}
	}
	return nil
}

func DBGetRewardRecordByPoolID(poolid string, limit int64) ([]shared.RewardRecord, error) {
	var result []shared.RewardRecord
	filter := bson.M{"dataid": bson.M{operator.Eq: poolid}}
	err := mgm.Coll(&shared.RewardRecord{}).SimpleFind(&result, filter, &options.FindOptions{
		Sort:  bson.D{{"beaconheight", -1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func DBDeleteRewardRecord(height uint64) error {
	filter := bson.M{"beaconheight": bson.M{operator.Lte: height}}
	_, err := mgm.Coll(&shared.RewardRecord{}).DeleteMany(context.Background(), filter)
	if err != nil {
		return err
	}
	return nil
}

func DBGetPDEPoolPairRewardAPY(poolid string) (*shared.RewardAPYTracking, error) {
	var result []shared.RewardAPYTracking
	filter := bson.M{"dataid": bson.M{operator.Eq: poolid}}
	err := mgm.Coll(&shared.RewardAPYTracking{}).SimpleFind(&result, filter)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return &result[0], nil
}

func DBGetLimitOrderStatusByPairID(pairid string) ([]shared.LimitOrderStatus, error) {
	var tradeStatus []shared.LimitOrderStatus
	filter := bson.M{"pairid": bson.M{operator.Eq: pairid}}
	err := mgm.Coll(&shared.LimitOrderStatus{}).SimpleFind(&tradeStatus, filter)
	if err != nil {
		return nil, err
	}
	return tradeStatus, nil
}
func DBGetPendingLimitOrderByNftID(nftIDs []string) ([]shared.TradeOrderData, error) {
	var orders []shared.LimitOrderStatus
	filter := bson.M{"nftid": bson.M{operator.In: nftIDs}}
	err := mgm.Coll(&shared.LimitOrderStatus{}).SimpleFind(&orders, filter)
	if err != nil {
		return nil, err
	}
	var requestTxs []string
	if len(orders) == 0 {
		return nil, nil
	}
	for _, v := range orders {
		requestTxs = append(requestTxs, v.RequestTx)
	}

	list := []shared.TradeOrderData{}
	filter = bson.M{"requesttx": bson.M{operator.In: requestTxs}}
	err = mgm.Coll(&shared.TradeOrderData{}).SimpleFind(&list, filter)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DBGetPendingLimitOrderByAccessOTA(accessOTAs []string) (map[string]shared.TradeOrderData, error) {
	var orders []shared.LimitOrderStatus
	filter := bson.M{"currentaccess": bson.M{operator.In: accessOTAs}}
	err := mgm.Coll(&shared.LimitOrderStatus{}).SimpleFind(&orders, filter)
	if err != nil {
		return nil, err
	}
	var requestTxs []string
	if len(orders) == 0 {
		return nil, nil
	}
	accessOTAMap := make(map[string]string)
	for _, v := range orders {
		requestTxs = append(requestTxs, v.RequestTx)
		accessOTAMap[v.RequestTx] = v.CurrentAccessID
	}

	result := make(map[string]shared.TradeOrderData)
	list := []shared.TradeOrderData{}
	filter = bson.M{"requesttx": bson.M{operator.In: requestTxs}}
	err = mgm.Coll(&shared.TradeOrderData{}).SimpleFind(&list, filter)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if accessOTA, ok := accessOTAMap[v.RequestTx]; ok {
			result[accessOTA] = v
		}
	}
	return result, nil
}
func DBGetSharByAccessOTA(accessOTAs []string) (map[string]shared.PoolShareData, error) {
	var list []shared.PoolShareData
	result := make(map[string]shared.PoolShareData)
	filter := bson.M{"currentaccess": bson.M{operator.In: accessOTAs}}
	err := mgm.Coll(&shared.PoolShareData{}).SimpleFind(&list, filter)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		result[v.CurrentAccessID] = v
	}
	return result, nil
}
