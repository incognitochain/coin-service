package liquidity

import (
	"context"

	"github.com/incognitochain/coin-service/database"
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
		"_id":      bson.M{operator.Gt: obID},
		"metatype": bson.M{operator.In: metas},
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

func updateState(state *State, statename string) error {
	result, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	return database.DBUpdateProcessorState(statename, string(result))
}

func loadState(state *State, statename string) error {
	result, err := database.DBGetProcessorState(statename)
	if err != nil {
		return err
	}
	if result == nil {
		state = &State{}
		return nil
	}
	return json.UnmarshalFromString(result.State, state)
}
