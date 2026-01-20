package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"patrn.ink/internal/config"
)

var Logger *zap.Logger

// InitLogger initializes the structured logger
func InitLogger() error {
	var err error

	if config.AppConfig.Environment == "production" {
		Logger, err = zap.NewProduction()
	} else {
		cfg := zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		Logger, err = cfg.Build()
	}

	if err != nil {
		return err
	}

	return nil
}
