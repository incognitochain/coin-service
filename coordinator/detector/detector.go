package detector

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/incognitochain/coin-service/logging/logger"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (dtc *Detector) StartService(port int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	logger.RegisterLoggerServer(s, dtc)
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (dtc *Detector) RecordLog(ctx context.Context, in *logger.LogRequest) (*emptypb.Empty, error) {
	log.Printf("Received: %v", in.Data)
	return new(emptypb.Empty), nil
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
