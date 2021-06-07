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
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBSaveTxShield(list []shared.ShieldData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, tx := range list {
		tx.Creating()
		docs = append(docs, tx)
	}
	_, err := mgm.Coll(&shared.ShieldData{}).InsertMany(ctx, docs)
	if err != nil {
		return err
	}
	return nil
}

func DBSaveTxUnShield(list []shared.ShieldData) error {
	return nil
}

func DBGetTxShieldRespond(pubkey string, limit int64, offset int64) ([]shared.TxData, error) {
	if limit == 0 {
		limit = int64(10000)
	}
	metas := []string{strconv.Itoa(metadata.IssuingResponseMeta), strconv.Itoa(metadata.IssuingETHResponseMeta)}
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

func DBGetTxShield(respondList []string) ([]shared.ShieldData, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(respondList)+1)*shared.DB_OPERATION_TIMEOUT)
	result := []shared.ShieldData{}

	filter := bson.M{"respondtx": bson.M{operator.In: respondList}}
	err := mgm.Coll(&shared.ShieldData{}).SimpleFindWithCtx(ctx, &result, filter)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return result, nil
}
