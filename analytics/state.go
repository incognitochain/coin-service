package analytics

import "github.com/incognitochain/coin-service/database"

var currentState State

func updateState(processor string) error {
	result, err := json.Marshal(currentState)
	if err != nil {
		panic(err)
	}
	return database.DBUpdateProcessorState(processor, string(result))
}

func loadState(processor string) error {
	result, err := database.DBGetProcessorState(processor)
	if err != nil {
		return err
	}
	if result == nil {
		currentState = State{}
		return nil
	}
	return json.UnmarshalFromString(result.State, &currentState)
}
