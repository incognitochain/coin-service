package logging

import (
	"log"
	"os"

	"github.com/rs/zerolog"
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
	logger.Info().Msgf("%s", p)
	return len(p), nil
}
