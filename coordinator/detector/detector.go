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

func (dtc *Detector) GetCrashReport() map[string]ServiceRecorder {
	dtc.Lck.RLock()
	defer dtc.Lck.RUnlock()
	return dtc.Services
}

func (dtc *Detector) GetCrashReportByService(serviceName string) map[string][]RecordDetail {
	dtc.Lck.RLock()
	defer dtc.Lck.RUnlock()
	if service, ok := dtc.Services[serviceName]; ok {
		return service.Records
	}
	return nil
}

func (dtc *Detector) GetCrashReportByServiceAndType(serviceName string, recordType string) []RecordDetail {
	dtc.Lck.RLock()
	defer dtc.Lck.RUnlock()
	if service, ok := dtc.Services[serviceName]; ok {
		if records, ok := service.Records[recordType]; ok {
			return records
		}
	}
	return nil
}

func (dtc *Detector) GetCrashCountByService(serviceName string) map[string]int {
	dtc.Lck.RLock()
	defer dtc.Lck.RUnlock()
	result := make(map[string]int)
	if service, ok := dtc.Services[serviceName]; ok {
		for k, v := range service.Records {
			result[k] = len(v)
		}
	}
	return result
}

func (dtc *Detector) AddRecord(record RecordDetail, serviceGroup string) {
	if dtc.Services == nil {
		dtc.Services = make(map[string]ServiceRecorder)
	}
	if _, ok := dtc.Services[serviceGroup]; !ok {
		dtc.Services[serviceGroup] = ServiceRecorder{}
	}
	dtc.Lck.Lock()
	defer dtc.Lck.Unlock()
	cs := dtc.Services[serviceGroup]
	if len(cs.Records) == 0 {
		cs.Records = make(map[string][]RecordDetail)
	}
	rlist := cs.Records[record.Type]
	rlist = append(rlist, RecordDetail{
		ServiceID: record.ServiceID,
		Reason:    record.Reason,
		Time:      time.Now().Unix(),
	})
	cs.Records[record.Type] = rlist
}
