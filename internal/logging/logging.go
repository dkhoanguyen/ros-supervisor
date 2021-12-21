package logging

import (
	"log"
	"os"
	"time"

	"github.com/dkhoanguyen/ros-supervisor/internal/env"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Make(config *env.Config) *zap.Logger {

	// Create log folder if it doesn't exist
	if _, err := os.Stat(config.LoggingPath); os.IsNotExist(err) {
		err := os.Mkdir(config.LoggingPath, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	}

	currentTime := time.Now().Format("2006-01-02-15-04-05")
	fileName := currentTime + ".log"

	// Open logfile
	logfile, err := os.OpenFile(config.LoggingPath+"/"+fileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0677)
	if err != nil {
		log.Fatal(err)
	}

	// Logwriter
	logWriter := zapcore.NewMultiWriteSyncer(os.Stdout, logfile)

	// Encoder
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "@timestamp"
	encoderCfg.EncodeTime = zapcore.EpochMillisTimeEncoder

	encoder := zapcore.NewConsoleEncoder(encoderCfg)

	logCore := zapcore.NewCore(encoder, logWriter, zapcore.InfoLevel)
	logg := zap.New(logCore, zap.AddCaller())

	logg.Info("Done")

	return logg
}
