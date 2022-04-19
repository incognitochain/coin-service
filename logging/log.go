package logging

import (
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

func InitLogger(recorderEndpoint string) {
	logRecorderEndpoint = &LogRecorder{Endpoint: recorderEndpoint}
	multi := zerolog.MultiLevelWriter(logRecorderEndpoint, os.Stdout)
	newLogger := zerolog.New(multi).With().Timestamp().Logger()
	logger = &newLogger
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(logger)
}

func (l *LogRecorder) Write(p []byte) (n int, err error) {
	if l.Endpoint != "" {
		logRecorderEndpoint.willPushLog(p)
	}
	return len(p), nil
}

func (l *LogRecorder) willPushLog(p []byte) {
	conn, err := grpc.Dial(l.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(fmt.Sprintf("did not connect: %v", err))
	}
	defer conn.Close()
	c := pblogger.NewLoggerClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = c.RecordLog(ctx, &pblogger.LogRequest{Data: p})
	if err != nil {
		panic(fmt.Sprintf("could not greet: %v", err))
	}
}
