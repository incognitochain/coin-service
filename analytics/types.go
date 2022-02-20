package analytics

import jsoniter "github.com/json-iterator/go"

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type State struct {
	LastProcessedObjectID string
}
