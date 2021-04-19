package database

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBGetSubmittedOTAKeys(bucketID int, offset int64) ([]shared.SubmittedOTAKeyData, error) {
	var result []shared.SubmittedOTAKeyData
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"bucketid": bson.M{operator.Eq: bucketID}}
	err := mgm.Coll(&shared.SubmittedOTAKeyData{}).SimpleFindWithCtx(ctx, &result, filter, &options.FindOptions{
		Sort: bson.D{{"created_at", 1}},
		Skip: &offset,
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return result, nil
}

func DBSaveSubmittedOTAKeys(keys []shared.SubmittedOTAKeyData) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*shared.DB_OPERATION_TIMEOUT)
	if len(keys) > 0 {
		ctx, _ = context.WithTimeout(context.Background(), time.Duration(len(keys))*shared.DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, key := range keys {
			key.Creating()
			key.Saving()
			docs = append(docs, key)
		}
		_, err := mgm.Coll(&shared.SubmittedOTAKeyData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func DBGetBucketStats(bucketSize int) (map[int]uint64, error) {
	result := make(map[int]uint64)
	d := mgm.Coll(&shared.SubmittedOTAKeyData{})
	for i := 0; i < bucketSize; i++ {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(10)*shared.DB_OPERATION_TIMEOUT)
		filter := bson.M{"bucketid": bson.M{operator.Eq: i}}
		count, err := d.CountDocuments(ctx, filter)
		if err != nil {
			return nil, err
		}
		result[i] = uint64(count)
	}

	return result, nil
}

func DBUpdateKeyInfoV2(doc interface{}, key *shared.KeyInfoData) error {
	ctx, _ := context.WithTimeout(context.Background(), 1*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"otasecret": bson.M{operator.Eq: key.OTAKey}}
	result, err := mgm.Coll(&shared.KeyInfoDataV2{}).UpdateOne(ctx, filter, doc, mgm.UpsertTrueOption())
	if err != nil {
		return err
	}
	if result.UpsertedID != nil {
		key.SetID(result.UpsertedID)
	}
	return nil
}
