package detector

import (
	"sync"

	"github.com/incognitochain/coin-service/logging/logger"
)

type Detector struct {
	Lck      sync.RWMutex
	Services map[string]ServiceCrashRecorder

	logger.UnimplementedLoggerServer
}

type ServiceCrashRecorder struct {
	LostConnectionCount int
	KnownCrash          []CrashReason
	UnknownCrash        []CrashReason
}

type CrashReason struct {
	ServiceID string
	Reason    string
	Time      int64
}
