package coordinator

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/incognitochain/coin-service/coordinator/detector"
	"github.com/incognitochain/coin-service/coordinator/mongodump"
	"github.com/incognitochain/coin-service/shared"
	"github.com/mongodb/mongo-tools/common/progress"
)

var state CoordinatorState

func init() {
	newDetector := detector.Detector{}
	state.Detector = &newDetector
	state.ConnectedServices = make(map[string]map[string]*ServiceConn)
}

func StartCoordinator() {
	fmt.Println("start coordinator")
	go state.Detector.StartAPI(shared.ServiceCfg.APIPort - 1)
	for {
		time.Sleep(5 * time.Second)
		updateAllServiceStatus()
		state.backStatusLock.RLock()
		//periodcally resume all services because they are paused by default on connected until a resume request is received
		if state.backupContext == nil {
			err := resumeAllServices()
			if err != nil {
				log.Println(err)
			}
			state.backStatusLock.RUnlock()
			continue
		}
		state.backStatusLock.RUnlock()
	}
}

func startBackup() {
	defer func() {
		state.backupCancelFn = nil
		state.backupContext = nil
		state.currentBackupProgress = nil
		state.backupState = 0
	}()

	for {
		if !checkIfServicePaused([]string{SERVICEGROUP_CHAINSYNKER, SERVICEGROUP_INDEXER, SERVICEGROUP_INDEXWORKER, SERVICEGROUP_LIQUIDITY_PROCESSOR, SERVICEGROUP_SHIELD_PROCESSOR, SERVICEGROUP_TRADE_PROCESSOR}) {
			state.backupState = 1
			if err := pauseServices([]string{SERVICEGROUP_CHAINSYNKER, SERVICEGROUP_INDEXER, SERVICEGROUP_INDEXWORKER, SERVICEGROUP_LIQUIDITY_PROCESSOR, SERVICEGROUP_SHIELD_PROCESSOR, SERVICEGROUP_TRADE_PROCESSOR}); err != nil {
				state.lastFailBackupErr = err.Error()
				state.lastFailBackupTime = time.Now()
			}
			time.Sleep(4 * time.Second)
		} else {
			state.backupState = 2
			log.Println("successfully pause all services")
			break
		}
	}

	backupResult := make(chan string, 1)
	mongoDumpProgress := &ProgressManager{
		Progress: make(map[string]progress.Progressor),
	}
	state.currentBackupProgress = mongoDumpProgress
	pwd, _ := os.Getwd()

	go mongodump.DumpMongo(state.backupContext, backupResult, shared.ServiceCfg.MongoAddress, path.Join(pwd, "mongodump"), mongoDumpProgress)
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
			state.backupState = 3
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

func updateAllServiceStatus() {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()
	for _, v := range state.ConnectedServices {
		for _, sv := range v {
			if err := sendRequestToService(sv, ACTION_OPERATION_MODE, "get"); err != nil {
				log.Printf("pause service %v with ID %v failed: %v\n", sv.ServiceName, sv.ID, err)
			}
		}
	}
}

func checkIfServicePaused(serviceGroups []string) bool {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()
	for _, serviceGroup := range serviceGroups {
		for _, sv := range state.ConnectedServices[serviceGroup] {
			if !sv.IsPause {
				return false
			}
		}
	}
	return true
}

func pauseServices(serviceGroups []string) error {
	state.ConnectedServicesLock.Lock()
	defer state.ConnectedServicesLock.Unlock()
	for _, serviceGroup := range serviceGroups {
		for _, v := range state.ConnectedServices[serviceGroup] {
			if err := sendRequestToService(v, ACTION_OPERATION_MODE, "pause"); err != nil {
				return fmt.Errorf("pause service %v with ID %v failed: %v\n", v.ServiceName, v.ID, err)
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
