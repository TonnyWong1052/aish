package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// Logger wraps logrus.Logger with additional functionality
type Logger struct {
	*logrus.Logger
	component string
}

var (
	// globalLogger global logger instance
	globalLogger *Logger
	// logFile log file handle
	logFile *os.File
)

// LogLevel log level type
type LogLevel string

const (
	TraceLevel LogLevel = "trace"
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	FatalLevel LogLevel = "fatal"
	PanicLevel LogLevel = "panic"
)

// Config log configuration
type Config struct {
	Level      LogLevel `json:"level"`       // Log level
	Format     string   `json:"format"`      // Format: "json" or "text"
	Output     string   `json:"output"`      // Output: "file", "console", "both"
	LogFile    string   `json:"log_file"`    // Log file path
	MaxSize    int64    `json:"max_size"`    // Maximum file size (MB)
	MaxBackups int      `json:"max_backups"` // Maximum backup file count
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Level:      InfoLevel,
		Format:     "text",
		Output:     "file",
		LogFile:    filepath.Join(home, ".config", "aish", "logs", "aish.log"),
		MaxSize:    10, // 10MB
		MaxBackups: 5,
	}
}

// Init initializes logging system
func Init(config Config) error {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(string(config.Level))
	if err != nil {
		return fmt.Errorf("invalid log level: %s", config.Level)
	}
	logger.SetLevel(level)

	// Set format
	switch config.Format {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "function",
				logrus.FieldKeyFile:  "file",
			},
		})
	case "text":
		logger.SetFormatter(&CustomTextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	default:
		return fmt.Errorf("invalid log format: %s", config.Format)
	}

	// Set output
	switch config.Output {
	case "console":
		logger.SetOutput(os.Stdout)
	case "file":
		if err := setupFileOutput(logger, config); err != nil {
			return fmt.Errorf("failed to setup file output: %w", err)
		}
	case "both":
		// Output to both console and file
		if err := setupFileOutput(logger, config); err != nil {
			return fmt.Errorf("failed to setup file output: %w", err)
		}
		// TODO: Implement multiple outputs (requires additional packages or custom implementation)
	default:
		return fmt.Errorf("invalid log output: %s", config.Output)
	}

	// Create global logger instance
	globalLogger = &Logger{
		Logger:    logger,
		component: "aish",
	}

	return nil
}

// setupFileOutput sets up file output
func setupFileOutput(logger *logrus.Logger, config Config) error {
	// Ensure log directory exists
	logDir := filepath.Dir(config.LogFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Open log file
	file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logFile = file
	logger.SetOutput(file)
	return nil
}

// GetLogger gets global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		// If not initialized, initialize with default config
		if err := Init(DefaultConfig()); err != nil {
			// If initialization fails, create a simple console logger
			logger := logrus.New()
			logger.SetOutput(os.Stderr)
			globalLogger = &Logger{
				Logger:    logger,
				component: "aish",
			}
		}
	}
	return globalLogger
}

// WithComponent creates logger instance with component identifier
func WithComponent(component string) *Logger {
	base := GetLogger()
	return &Logger{
		Logger:    base.Logger,
		component: component,
	}
}

// WithField adds field
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{
		"component": l.component,
		key:         value,
	})
}

// WithFields adds multiple fields
func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	fields["component"] = l.component
	return l.Logger.WithFields(fields)
}

// WithError adds error field
func (l *Logger) WithError(err error) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields{
		"component": l.component,
		"error":     err,
	})
}

// Convenience methods
func (l *Logger) Trace(args ...interface{}) {
	l.WithFields(logrus.Fields{}).Trace(args...)
}

func (l *Logger) Debug(args ...interface{}) {
	l.WithFields(logrus.Fields{}).Debug(args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.WithFields(logrus.Fields{}).Info(args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.WithFields(logrus.Fields{}).Warn(args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.WithFields(logrus.Fields{}).Error(args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.WithFields(logrus.Fields{}).Fatal(args...)
}

func (l *Logger) Panic(args ...interface{}) {
	l.WithFields(logrus.Fields{}).Panic(args...)
}

// Tracef formats trace log
func (l *Logger) Tracef(format string, args ...interface{}) {
	l.WithFields(logrus.Fields{}).Tracef(format, args...)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.WithFields(logrus.Fields{}).Debugf(format, args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.WithFields(logrus.Fields{}).Infof(format, args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.WithFields(logrus.Fields{}).Warnf(format, args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.WithFields(logrus.Fields{}).Errorf(format, args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.WithFields(logrus.Fields{}).Fatalf(format, args...)
}

func (l *Logger) Panicf(format string, args ...interface{}) {
	l.WithFields(logrus.Fields{}).Panicf(format, args...)
}

// Close closes logging system
func Close() error {
	if logFile != nil {
		return logFile.Close()
	}
	return nil
}

// SetLevel dynamically sets log level
func SetLevel(level LogLevel) error {
	logger := GetLogger()
	logrusLevel, err := logrus.ParseLevel(string(level))
	if err != nil {
		return err
	}
	logger.SetLevel(logrusLevel)
	return nil
}

// GetLevel gets current log level
func GetLevel() LogLevel {
	logger := GetLogger()
	return LogLevel(logger.GetLevel().String())
}

// Global convenience functions
func Trace(args ...interface{}) {
	GetLogger().Trace(args...)
}

func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

func Panic(args ...interface{}) {
	GetLogger().Panic(args...)
}

func Tracef(format string, args ...interface{}) {
	GetLogger().Tracef(format, args...)
}

func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

func Panicf(format string, args ...interface{}) {
	GetLogger().Panicf(format, args...)
}

func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}

func WithError(err error) *logrus.Entry {
	return GetLogger().WithError(err)
}

// CustomTextFormatter custom text formatter
type CustomTextFormatter struct {
	TimestampFormat string
	FullTimestamp   bool
}

// Format implements logrus.Formatter interface
func (f *CustomTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b strings.Builder

	// Timestamp
	if f.FullTimestamp {
		b.WriteString(entry.Time.Format(f.TimestampFormat))
		b.WriteString(" ")
	}

	// Log level
	b.WriteString("[")
	b.WriteString(strings.ToUpper(entry.Level.String()))
	b.WriteString("] ")

	// Component information
	if component, ok := entry.Data["component"].(string); ok {
		b.WriteString("[")
		b.WriteString(component)
		b.WriteString("] ")
	}

	// Call location (only shown at debug level)
	if entry.Level <= logrus.DebugLevel && entry.HasCaller() {
		_, file, line, ok := runtime.Caller(8)
		if ok {
			b.WriteString(fmt.Sprintf("[%s:%d] ", filepath.Base(file), line))
		}
	}

	// Message
	b.WriteString(entry.Message)

	// Additional fields
	for key, value := range entry.Data {
		if key != "component" {
			b.WriteString(fmt.Sprintf(" %s=%v", key, value))
		}
	}

	b.WriteString("\n")
	return []byte(b.String()), nil
}
