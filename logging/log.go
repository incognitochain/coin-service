package logging

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	pblogger "github.com/incognitochain/coin-service/logging/logger"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var logger *zerolog.Logger
var logRecorderEndpoint *LogRecorder
var logClient *pblogger.LoggerClient
var logClientConn *grpc.ClientConn
var separatorBytes = []byte("|")
var serviceID string
var serviceGroup string

func InitLogger(recorderEndpoint string, sID string, sGroup string) {
	serviceID = sID
	serviceGroup = sGroup
	logRecorderEndpoint = &LogRecorder{Endpoint: recorderEndpoint}
	multi := zerolog.MultiLevelWriter(logRecorderEndpoint, os.Stdout)
	newLogger := zerolog.New(multi).With().Timestamp().Logger()
	logger = &newLogger
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(logger)
	go connectLogRecorder(logRecorderEndpoint.Endpoint)
}

func (l *LogRecorder) Write(p []byte) (n int, err error) {
	if l.Endpoint != "" {
		go logRecorderEndpoint.willPushLog(p)
	}
	return len(p), nil
}

func (l *LogRecorder) willPushLog(p []byte) {
	logType := bytes.Split(p, separatorBytes)
	data := &pblogger.LogRequest{Data: p, Type: logType[0], ServiceId: []byte(serviceID), ServiceGroup: []byte(serviceGroup), Timestamp: time.Now().Unix()}
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in willPushLog", r)
			connectLogRecorder(logRecorderEndpoint.Endpoint)
		}
	}()
retry:
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if logClient == nil {
		connectLogRecorder(logRecorderEndpoint.Endpoint)
	}
	c := *logClient
	_, err := c.RecordLog(ctx, data)
	if err != nil {
		// log.Printf("could not record log: %v", err)
		time.Sleep(time.Second)
		goto retry
	}
}

func connectLogRecorder(endpoint string) {
	if logClientConn != nil {
		logClientConn.Close()
	}
retry:
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to dial: %v", err)
		time.Sleep(time.Second)
		goto retry
	}
	c := pblogger.NewLoggerClient(conn)
	logClientConn = conn
	logClient = &c
}
