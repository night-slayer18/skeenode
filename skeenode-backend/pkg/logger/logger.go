package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	once         sync.Once
)

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Encoding   string // json or console
	OutputPath string // stdout, stderr, or file path
	Service    string // service name for log context
}

// DefaultConfig returns production-ready defaults
func DefaultConfig(service string) Config {
	return Config{
		Level:      "info",
		Encoding:   "json",
		OutputPath: "stdout",
		Service:    service,
	}
}

// Init initializes the global logger with the given configuration
func Init(cfg Config) (*zap.Logger, error) {
	var err error
	once.Do(func() {
		globalLogger, err = newLogger(cfg)
	})
	return globalLogger, err
}

// Get returns the global logger, initializing with defaults if needed
func Get() *zap.Logger {
	if globalLogger == nil {
		cfg := DefaultConfig("skeenode")
		logger, _ := newLogger(cfg)
		globalLogger = logger
	}
	return globalLogger
}

// newLogger creates a new zap logger with the given configuration
func newLogger(cfg Config) (*zap.Logger, error) {
	level := parseLevel(cfg.Level)

	encoderConfig := zapcore.EncoderConfig{
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
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if cfg.Encoding == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	var output zapcore.WriteSyncer
	switch cfg.OutputPath {
	case "stdout":
		output = zapcore.AddSync(os.Stdout)
	case "stderr":
		output = zapcore.AddSync(os.Stderr)
	default:
		file, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		output = zapcore.AddSync(file)
	}

	core := zapcore.NewCore(encoder, output, level)
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.Fields(zap.String("service", cfg.Service)),
	)

	return logger, nil
}

// parseLevel converts string to zapcore.Level
func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// WithFields returns a new logger with additional fields
func WithFields(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}

// Info logs an info message with optional fields
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Error logs an error message with optional fields
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Warn logs a warning message with optional fields
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Debug logs a debug message with optional fields
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}
