package database

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBSaveTokenInfo(list []shared.TokenInfoData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, coin := range list {
		coin.Creating()
		docs = append(docs, coin)
	}
	_, err := mgm.Coll(&list[0]).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
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
				_, err = mgm.Coll(&shared.TokenInfoData{}).InsertOne(ctx, v)
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
	log.Printf("inserted %v keyimages in %v", len(list), time.Since(startTime))
	return nil
}

func DBGetTokenCount() (int64, error) {
	filter := bson.M{"isnft": bson.M{operator.Eq: false}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	c, err := mgm.Coll(&shared.TokenInfoData{}).Collection.CountDocuments(ctx, filter)
	if err != nil {
		return c, err
	}
	return c, nil
}

func DBGetAllTokenInfo() ([]shared.TokenInfoData, error) {
	list := []shared.TokenInfoData{}
	filter := bson.M{"isnft": bson.M{operator.Eq: false}}
	err := mgm.Coll(&shared.TokenInfoData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DBGetNFTCount() (int64, error) {
	filter := bson.M{"isnft": bson.M{operator.Eq: true}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	c, err := mgm.Coll(&shared.TokenInfoData{}).Collection.CountDocuments(ctx, filter)
	if err != nil {
		return c, err
	}
	return c, nil
}
func DBGetNFTInfo() ([]shared.TokenInfoData, error) {
	list := []shared.TokenInfoData{}
	filter := bson.M{"isnft": bson.M{operator.Eq: true}}
	err := mgm.Coll(&shared.TokenInfoData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DBGetTokenByTokenID(tokenids []string) ([]shared.TokenInfoData, error) {
	list := []shared.TokenInfoData{}
	filter := bson.M{"tokenid": bson.M{operator.In: tokenids}}
	err := mgm.Coll(&shared.TokenInfoData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DBSaveExtraTokenInfo(list []shared.ExtraTokenInfo) error {
	docs := []interface{}{}
	for _, tx := range list {
		update := bson.M{
			"$set": tx,
		}
		docs = append(docs, update)
	}
	for idx, v := range list {
		filter := bson.M{"tokenid": bson.M{operator.Eq: v.TokenID}}
		_, err := mgm.Coll(&shared.ExtraTokenInfo{}).UpdateOne(context.Background(), filter, docs[idx], mgm.UpsertTrueOption())
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
	return nil
}

func DBSaveCustomTokenInfo(list []shared.CustomTokenInfo) error {
	docs := []interface{}{}
	for _, tx := range list {
		update := bson.M{
			"$set": tx,
		}
		docs = append(docs, update)
	}
	for idx, v := range list {
		filter := bson.M{"tokenid": bson.M{operator.Eq: v.TokenID}}
		_, err := mgm.Coll(&shared.CustomTokenInfo{}).UpdateOne(context.Background(), filter, docs[idx], mgm.UpsertTrueOption())
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
	return nil
}

func DBGetAllCustomTokenInfo() ([]shared.CustomTokenInfo, error) {
	list := []shared.CustomTokenInfo{}
	filter := bson.M{}
	err := mgm.Coll(&shared.CustomTokenInfo{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	return list, nil
}
func DBGetCustomTokenInfoByTokenID(tokenids []string) ([]shared.CustomTokenInfo, error) {
	list := []shared.CustomTokenInfo{}
	filter := bson.M{"tokenid": bson.M{operator.In: tokenids}}
	err := mgm.Coll(&shared.CustomTokenInfo{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DBGetExtraTokenInfoByTokenID(tokenids []string) ([]shared.ExtraTokenInfo, error) {
	list := []shared.ExtraTokenInfo{}
	filter := bson.M{"tokenid": bson.M{operator.In: tokenids}}
	err := mgm.Coll(&shared.ExtraTokenInfo{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DBGetAllExtraTokenInfo() ([]shared.ExtraTokenInfo, error) {
	list := []shared.ExtraTokenInfo{}
	filter := bson.M{}
	err := mgm.Coll(&shared.ExtraTokenInfo{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DBGetExtraTokenInfo(tokenID string) (*shared.ExtraTokenInfo, error) {
	var result []shared.ExtraTokenInfo
	filter := bson.M{"tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&shared.ExtraTokenInfo{}).SimpleFind(&result, filter, nil)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return &result[0], nil
}

func DBUpdateTokenInfoPrice(tokenList []shared.TokenInfoData) error {
	docs := []interface{}{}
	for _, tk := range tokenList {
		update := bson.M{
			"$set": bson.M{
				"updated_at":   tk.UpdatedAt,
				"pastprice":    tk.PastPrice,
				"currentprice": tk.CurrentPrice,
			},
		}
		docs = append(docs, update)
	}
	for idx, v := range tokenList {
		filter := bson.M{"tokenid": bson.M{operator.Eq: v.TokenID}}
		_, err := mgm.Coll(&shared.TokenInfoData{}).UpdateOne(context.Background(), filter, docs[idx])
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
	return nil
}
