package database

import (
	"context"
	"encoding/base64"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
)

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
