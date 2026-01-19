package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// InitLogger initializes the structured logger
func InitLogger() error {
	var err error

	if AppConfig.Environment == "production" {
		Logger, err = zap.NewProduction()
	} else {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		Logger, err = config.Build()
	}

	if err != nil {
		return err
	}

	return nil
}
