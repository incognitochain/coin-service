package coordinator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/incognitochain/coin-service/coordinator/mongodump"
)

var state CoordinatorState

func init() {
	state.ConnectedServices = make(map[string]map[string]*ServiceConn)
}

func StartCoordinator() {

}

func startBackup(ctx context.Context) {
	defer func() {
		state.backupCancelFn = nil
		state.backupContext = nil
	}()

	for {
		if !checkIfAllServicePaused() {
			if err := pauseService(); err != nil {
				state.lastFailBackupErr = err
				state.lastFailBackupTime = time.Now()
			}
		} else {
			log.Println("successfully pause all services")
			break
		}
	}

	backupSuccessCtx, backSuccessFn := context.WithCancel(ctx)
	var backupErr error
	go func() {
		backupErr = mongodump.DumpMongo(state.backupContext, backSuccessFn)
	}()
	for {
		select {
		case <-backupSuccessCtx.Done():
			if backupErr != nil {
				state.lastFailBackupErr = backupErr
				state.lastFailBackupTime = time.Now()
			} else {
				state.lastSuccessBackupTime = time.Now()
			}
		case <-ctx.Done():
			return
		}
	}
}

func checkIfAllServicePaused() bool {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()
	for _, v := range state.ConnectedServices {
		for _, sv := range v {
			if !sv.IsPause {
				return false
			}
		}
	}
	return true
}

func pauseService() error {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()
	for _, v := range state.ConnectedServices {
		for _, sv := range v {
			if err := sendPauseRequest(sv); err != nil {
				return fmt.Errorf("pause service %v with ID %v failed: %v\n", sv.ServiceName, sv.ID, err)
			}
		}
	}
	return nil
}

func sendPauseRequest(sv *ServiceConn) error {
	return nil
}

func registerService(sv *ServiceConn) error {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()

	if _, ok := state.ConnectedServices[sv.ServiceName]; ok {
		if _, ok1 := state.ConnectedServices[sv.ServiceName][sv.ID]; ok1 {
			return errors.New("serviceID already exist")
		}
	} else {
		state.ConnectedServices[sv.ServiceName] = make(map[string]*ServiceConn)
	}
	state.ConnectedServices[sv.ServiceName][sv.ID] = sv
	log.Printf("register service %v with ID %v success\n", sv.ServiceName, sv.ID)
	return nil
}

func removeService(sv *ServiceConn) error {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()

	if _, ok := state.ConnectedServices[sv.ServiceName]; ok {
		delete(state.ConnectedServices[sv.ServiceName], sv.ID)
	} else {
		return errors.New("service not exist")
	}
	return nil
}
