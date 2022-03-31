package coordinator

import (
	"context"
	"sync"
	"time"
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
	lastSuccessBackupTime time.Time

	lastFailBackupTime time.Time
	lastFailBackupErr  error
}

type ServiceConn struct {
	ServiceName string
	ID          string
	Heartbeat   int64
	IsPause     bool
	readCh      chan []byte
	writeCh     chan []byte
	closeCh     chan struct{}
}
