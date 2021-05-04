package database

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson"
)

func DBSaveTokenInfo(list []shared.TokenInfoData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{}
	lengthTokens, err := mgm.Coll(&shared.TokenInfoData{}).CountDocuments(ctx, filter)
	if err != nil {
		log.Println(err)
		return err
	}
	if lengthTokens != int64(len(list)) {
		ctx, _ = context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
		err = mgm.Coll(&shared.TokenInfoData{}).Drop(ctx)
		if err != nil {
			log.Println(err)
			return err
		}
		if len(list) > 0 {
			ctx, _ = context.WithTimeout(context.Background(), time.Duration(len(list)+1)*shared.DB_OPERATION_TIMEOUT)
			docs := []interface{}{}
			for _, token := range list {
				token.Creating()
				docs = append(docs, token)
			}
			_, err = mgm.Coll(&shared.TokenInfoData{}).InsertMany(ctx, docs)
			if err != nil {
				log.Println(err)
				return err
			}
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
