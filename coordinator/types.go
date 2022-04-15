package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/coordinator/detector"
	"github.com/mongodb/mongo-tools/common/progress"
)

type CoordinatorCmd struct {
	Action string
	Data   string
}

type CoordinatorState struct {
	Detector              *detector.Detector
	ConnectedServicesLock sync.RWMutex
	ConnectedServices     map[string]map[string]*ServiceConn

	backStatusLock        sync.RWMutex
	backupContext         context.Context
	backupCancelFn        context.CancelFunc
	currentBackupProgress *ProgressManager
	lastSuccessBackupTime time.Time

	lastFailBackupTime time.Time
	lastFailBackupErr  string
}

type ServiceConn struct {
	ServiceName   string
	ID            string
	IsPause       bool
	ReadCh        chan []byte
	WriteCh       chan []byte
	closeCh       chan struct{}
	ConnectedTime int64
}

type ProgressManager struct {
	Progress     map[string]progress.Progressor
	ProgressLock sync.RWMutex
}
