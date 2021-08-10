package database

import (
	"context"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
)

func DBSaveTokenInfo(list []shared.TokenInfoData) error {
	for _, v := range list {
		filter := bson.M{"tokenid": bson.M{operator.Eq: v.TokenID}}
		doc := bson.M{
			"$set": v,
		}
		_, err := mgm.Coll(&shared.TokenInfoData{}).UpdateOne(context.Background(), filter, doc, mgm.UpsertTrueOption())
		if err != nil {
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
