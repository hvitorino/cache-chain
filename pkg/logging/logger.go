package logging

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
}

// Config holds logging configuration
type Config struct {
	// Level is the log level (debug, info, warn, error, dpanic, panic, fatal)
	Level string
	// Format is the log format (json or console)
	Format string
	// OutputPaths is a list of paths to write logs to
	OutputPaths []string
	// ErrorOutputPaths is a list of paths to write internal logger errors to
	ErrorOutputPaths []string
	// Development enables development mode (DPanic logs will panic)
	Development bool
	// EnableCaller enables caller information in logs
	EnableCaller bool
	// EnableStacktrace enables stack traces for error logs
	EnableStacktrace bool
}

// DefaultConfig returns a default logging configuration
func DefaultConfig() Config {
	return Config{
		Level:            "info",
		Format:           "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		Development:      false,
		EnableCaller:     false,
		EnableStacktrace: false,
	}
}

// DevelopmentConfig returns a configuration for development
func DevelopmentConfig() Config {
	return Config{
		Level:            "debug",
		Format:           "console",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		Development:      true,
		EnableCaller:     true,
		EnableStacktrace: true,
	}
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config Config) (*Logger, error) {
	// Parse level
	level, err := parseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	if config.Development {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
	}
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	// Build zap config
	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       config.Development,
		DisableCaller:     !config.EnableCaller,
		DisableStacktrace: !config.EnableStacktrace,
		Sampling:          nil,
		Encoding:          config.Format,
		EncoderConfig:     encoderConfig,
		OutputPaths:       config.OutputPaths,
		ErrorOutputPaths:  config.ErrorOutputPaths,
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{logger}, nil
}

// NewLoggerFromEnv creates a logger based on environment variables
// LOG_LEVEL: log level (default: info)
// LOG_FORMAT: log format (default: json)
// LOG_DEV: enable development mode (default: false)
func NewLoggerFromEnv() (*Logger, error) {
	config := DefaultConfig()

	// Override from environment
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Format = format
	}
	if os.Getenv("LOG_DEV") == "true" {
		config = DevelopmentConfig()
		// Still allow overriding level in dev mode
		if level := os.Getenv("LOG_LEVEL"); level != "" {
			config.Level = level
		}
	}

	return NewLogger(config)
}

// NewNoOpLogger creates a logger that discards all logs
func NewNoOpLogger() *Logger {
	return &Logger{zap.NewNop()}
}

// parseLevel converts a string to a zapcore.Level
func parseLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "dpanic":
		return zapcore.DPanicLevel, nil
	case "panic":
		return zapcore.PanicLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, nil
	}
}

// With creates a child logger with additional fields
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{l.Logger.With(fields...)}
}

// Named creates a child logger with a name
func (l *Logger) Named(name string) *Logger {
	return &Logger{l.Logger.Named(name)}
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// Global logger instance
var global *Logger

func init() {
	// Initialize with a no-op logger
	global = NewNoOpLogger()
}

// SetGlobal sets the global logger instance
func SetGlobal(logger *Logger) {
	global = logger
}

// Global returns the global logger instance
func Global() *Logger {
	return global
}

// L returns the global logger instance (short form)
func L() *Logger {
	return global
}
