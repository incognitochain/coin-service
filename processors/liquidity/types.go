package liquidity

import jsoniter "github.com/json-iterator/go"

type State struct {
	LastProcessedObjectID string
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary