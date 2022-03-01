package trade

import (
	"context"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getTxToProcess(metas []string, lastID string, limit int64) ([]shared.TxData, error) {
	var result []shared.TxData
	var obID primitive.ObjectID
	if lastID == "" {
		obID = primitive.ObjectID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	} else {
		var err error
		obID, err = primitive.ObjectIDFromHex(lastID)
		if err != nil {
			return nil, err
		}
	}
	filter := bson.M{
		"_id":       bson.M{operator.Gt: obID},
		"metatype":  bson.M{operator.In: metas},
		"txversion": bson.M{operator.Eq: 2},
	}
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(context.Background(), &result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
