package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/mongodb/mongo-tools/common/progress"
)

type CoordinatorCmd struct {
	Action string
	Data   string
}

type CoordinatorState struct {
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
	ServiceName string
	ID          string
	IsPause     bool
	ReadCh      chan []byte
	WriteCh     chan []byte
	closeCh     chan struct{}
}

type ProgressManager struct {
	Progress progress.Progressor
}
