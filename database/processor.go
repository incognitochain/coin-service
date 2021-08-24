package database

import (
	"context"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func DBUpdateProcessorState(processor string, state string) error {
	fitler := bson.M{"processor": bson.M{operator.Eq: processor}}
	update := bson.M{
		"$set": bson.M{"state": state, "processor": processor},
	}
	err := mgm.Coll(&shared.ProcessorState{}).FindOneAndUpdate(context.Background(), fitler, update, options.FindOneAndUpdate().SetUpsert(true))
	if err != nil {
		return err.Err()
	}
	return nil
}

func DBGetProcessorState(processor string) (*shared.ProcessorState, error) {
	var result shared.ProcessorState
	filter := bson.M{"processor": bson.M{operator.Eq: processor}}
	err := mgm.Coll(&shared.ProcessorState{}).SimpleFind(result, filter)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
