package database

import (
	"context"
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

func DBSaveTxShield(list []shared.ShieldData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.ShieldData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.ShieldData{}).InsertOne(ctx, v)
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

func DBSaveTxUnShield(list []shared.ShieldData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.ShieldData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.ShieldData{}).InsertOne(ctx, v)
				if err != nil {
					writeErr, ok := err.(mongo.BulkWriteException)
					if !ok {
						panic(err)
					}
					er := writeErr.WriteErrors[0]
					if er.WriteError.Code != 11000 {
						panic(err)
					}
				}
			}
		}
	}
	return nil
}

func DBGetTxShield(pubkey, tokenID string, limit int64, offset int64) ([]shared.ShieldData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	var result []shared.ShieldData
	filter := bson.M{"pubkey": bson.M{operator.Eq: pubkey}, "tokenid": bson.M{operator.Eq: tokenID}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.ShieldData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"requesttime", -1}},
		Skip:  &offset,
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBGetTxUnshield(pubkey, tokenID string, limit int64, offset int64) ([]shared.TxData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	metas := []string{strconv.Itoa(metadata.BurningRequestMeta), strconv.Itoa(metadata.BurningRequestMetaV2), strconv.Itoa(metadata.BurningForDepositToSCRequestMeta), strconv.Itoa(metadata.BurningForDepositToSCRequestMetaV2), strconv.Itoa(metadata.ContractingRequestMeta), strconv.Itoa(metadata.BurningPBSCRequestMeta)}
	var result []shared.TxData
	filter := bson.M{"pubkeyreceivers": bson.M{operator.Eq: pubkey}, "realtokenid": bson.M{operator.Eq: tokenID}, "metatype": bson.M{operator.In: metas}}
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
func DBUpdateShieldData(list []shared.ShieldData) error {
	if len(list) == 0 {
		return nil
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	for _, tx := range list {
		fitler := bson.M{"requesttx": bson.M{operator.Eq: tx.RequestTx}}
		update := bson.M{
			"$set": bson.M{"requesttx": tx.RequestTx, "amount": tx.Amount, "respondtx": tx.RespondTx},
		}
		_, err := mgm.Coll(&shared.ShieldData{}).UpdateOne(ctx, fitler, update)
		if err != nil {
			return err
		}
	}
	return nil
}

func DBGetShieldWithRespond(fromtime uint64, offset int64) ([]shared.TxData, error) {
	var result []shared.TxData
	query := []string{strconv.Itoa(metadata.IssuingResponseMeta), strconv.Itoa(metadata.IssuingETHResponseMeta), strconv.Itoa(metadata.IssuingBSCResponseMeta), strconv.Itoa(metadataCommon.IssuingPRVERC20ResponseMeta), strconv.Itoa(metadataCommon.IssuingPRVBEP20ResponseMeta)}
	limit := int64(1000)
	filter := bson.M{"metatype": bson.M{operator.In: query}, "locktime": bson.M{operator.Gt: fromtime}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*shared.DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort:  bson.D{{"locktime", 1}},
		Limit: &limit,
		Skip:  &offset,
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DBSaveBridgeState(state string, height uint64, version int) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(10)*shared.DB_OPERATION_TIMEOUT)

	doc := bson.M{
		"$set": bson.M{
			"version": version,
			"state":   state,
			"height":  height,
		},
	}
	filter := bson.M{"version": bson.M{operator.Eq: version}}
	_, err := mgm.Coll(&shared.BridgeStateData{}).UpdateOne(ctx, filter, doc, mgm.UpsertTrueOption())
	if err != nil {
		return err
	}
	return nil
}

func DBGetBridgeState(version int) (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(4)*shared.DB_OPERATION_TIMEOUT)
	var result shared.BridgeStateData
	err := mgm.Coll(&shared.BridgeStateData{}).FirstWithCtx(ctx, bson.M{"version": bson.M{operator.Eq: version}}, &result)
	if err != nil {
		log.Println(err)
		return "", err
	}
	return result.State, nil
}
