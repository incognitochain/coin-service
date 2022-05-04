package detector

import (
	"sync"

	"github.com/incognitochain/coin-service/logging/logger"
)

type Detector struct {
	Lck      sync.RWMutex
	Services map[string]ServiceRecorder

	logger.UnimplementedLoggerServer
}

type ServiceRecorder struct {
	Records map[string][]RecordDetail // map[RECORD_TYPE][]RecordDetail
}

type RecordDetail struct {
	ServiceID string
	Type      string
	Reason    string
	Time      int64
}
