package analytics

import "github.com/incognitochain/coin-service/database"

var currentState State

func updateState() error {
	result, err := json.Marshal(currentState)
	if err != nil {
		panic(err)
	}
	return database.DBUpdateProcessorState("trade", string(result))
}

func loadState() error {
	result, err := database.DBGetProcessorState("trade")
	if err != nil {
		return err
	}
	if result == nil {
		currentState = State{}
		return nil
	}
	return json.UnmarshalFromString(result.State, &currentState)
}
