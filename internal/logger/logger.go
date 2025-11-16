package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger creates a new logger based on environment
func NewLogger() (*zap.Logger, error) {
	env := strings.ToLower(os.Getenv("ENV"))
	if env == "" {
		env = "development"
	}

	// Configure outputs
	var cores []zapcore.Core

	// Console output
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	if env == "production" {
		consoleEncoder = zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
		consoleEncoder = zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
	} else {
		consoleEncoder = zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
	}

	// Always add console output
	cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zap.InfoLevel))

	// File output if LOG_FILE is set
	logFile := os.Getenv("LOG_FILE")
	if logFile != "" {
		// Create lumberjack logger for file rotation
		lumberjackLogger := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    100, // megabytes
			MaxBackups: 3,   // number of backups
			MaxAge:     28,  // days
			Compress:   true,
		}

		fileEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})

		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(lumberjackLogger), zap.InfoLevel))
	}

	// Set log level from environment
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	atomicLevel := zap.NewAtomicLevel()
	if logLevel != "" {
		var level zapcore.Level
		switch logLevel {
		case "debug":
			level = zapcore.DebugLevel
		case "info":
			level = zapcore.InfoLevel
		case "warn", "warning":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		case "fatal":
			level = zapcore.FatalLevel
		default:
			level = zapcore.InfoLevel
		}
		atomicLevel.SetLevel(level)
	} else {
		atomicLevel.SetLevel(zapcore.InfoLevel)
	}

	// Create multi-core logger
	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// WithContext creates a logger with context fields
func WithContext(logger *zap.Logger, fields ...zap.Field) *zap.Logger {
	return logger.With(fields...)
}

// LogDuration performance logging helper
func LogDuration(logger *zap.Logger, operation string, duration int64, fields ...zap.Field) {
	fields = append(fields, zap.String("operation", operation), zap.Int64("duration_ms", duration))
	logger.Info("operation completed", fields...)
}
