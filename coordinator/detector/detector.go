package detector

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/logging/logger"
)

func (dtc *Detector) StartAPI(port int) {

}

func (dtc *Detector) RecordLog(ctx context.Context, in *logger.LogRequest) (*logger.LogReply, error) {
	log.Printf("Received: %v", in.Data)
	return &logger.LogReply{Message: "Hello " + in.Data}, nil
}

func (dtc *Detector) GetCrashReport() map[string]ServiceCrashRecorder {
	dtc.Lck.RLock()
	defer dtc.Lck.RUnlock()
	return dtc.Services
}

func (dtc *Detector) AddKnownCrashRecord(serviceID, reason string) {
	if dtc.Services == nil {
		dtc.Services = make(map[string]ServiceCrashRecorder)
	}
	if _, ok := dtc.Services[serviceID]; !ok {
		dtc.Services[serviceID] = ServiceCrashRecorder{}
	}
	dtc.Lck.Lock()
	defer dtc.Lck.Unlock()
	cs := dtc.Services[serviceID]
	cs.KnownCrash = append(cs.KnownCrash, CrashReason{
		ServiceID: serviceID,
		Reason:    reason,
		Time:      time.Now().Unix(),
	})
}

func (dtc *Detector) AddUnknownCrashRecord(serviceID, reason string) {
	if dtc.Services == nil {
		dtc.Services = make(map[string]ServiceCrashRecorder)
	}
	if _, ok := dtc.Services[serviceID]; !ok {
		dtc.Services[serviceID] = ServiceCrashRecorder{}
	}
	dtc.Lck.Lock()
	defer dtc.Lck.Unlock()
	cs := dtc.Services[serviceID]
	cs.UnknownCrash = append(cs.UnknownCrash, CrashReason{
		ServiceID: serviceID,
		Reason:    reason,
		Time:      time.Now().Unix(),
	})
}
