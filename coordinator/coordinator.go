package coordinator

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/incognitochain/coin-service/coordinator/mongodump"
	"github.com/incognitochain/coin-service/shared"
)

var state CoordinatorState

func init() {
	state.ConnectedServices = make(map[string]map[string]*ServiceConn)
}

func StartCoordinator() {
	fmt.Println("start coordinator")
}

func startBackup() {
	defer func() {
		state.backupCancelFn = nil
		state.backupContext = nil
		state.currentBackupProgress = nil
	}()

	for {
		if !checkIfAllServicePaused() {
			if err := pauseAllServices(); err != nil {
				state.lastFailBackupErr = err.Error()
				state.lastFailBackupTime = time.Now()
			}
			time.Sleep(4 * time.Second)
		} else {
			log.Println("successfully pause all services")
			break
		}
	}

	backupResult := make(chan string, 1)
	mongoDumpProgress := &ProgressManager{}
	state.currentBackupProgress = mongoDumpProgress
	go mongodump.DumpMongo(state.backupContext, backupResult, shared.ServiceCfg.MongoAddress, mongoDumpProgress)
	defer close(backupResult)

	for {
		select {
		case r := <-backupResult:
			if strings.Contains(r, "error") {
				state.lastFailBackupErr = r
				state.lastFailBackupTime = time.Now()
				log.Println("lastFailBackupErr", r)
			}
			if strings.Contains(r, "success") {
				state.lastSuccessBackupTime = time.Now()
				log.Println("dump success")
			}
			err := resumeAllServices()
			if err != nil {
				log.Println("resume all services failed:", err)
			}
			log.Println("all services resumed")
			state.backupCancelFn()
			return
		case <-state.backupContext.Done():
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

func pauseAllServices() error {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()
	for _, v := range state.ConnectedServices {
		for _, sv := range v {
			if err := sendRequestToService(sv, ACTION_OPERATION_MODE, "pause"); err != nil {
				return fmt.Errorf("pause service %v with ID %v failed: %v\n", sv.ServiceName, sv.ID, err)
			}
		}
	}
	return nil
}

func resumeAllServices() error {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()

	for {
		log.Println("trying resume all services...")
		allServiceResume := true
		for _, v := range state.ConnectedServices {
			for _, sv := range v {
				if !sv.IsPause {
					continue
				}
				allServiceResume = false
				if err := sendRequestToService(sv, ACTION_OPERATION_MODE, "resume"); err != nil {
					return fmt.Errorf("pause service %v with ID %v failed: %v\n", sv.ServiceName, sv.ID, err)
				}
			}
		}
		if allServiceResume {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
}

func sendRequestToService(sv *ServiceConn, action, data string) error {
	msg := CoordinatorCmd{
		Action: action,
		Data:   data,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	sv.WriteCh <- msgBytes
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
