package logging

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Level != InfoLevel {
		t.Errorf("Expected default level InfoLevel, got %v", config.Level)
	}

	if config.Format != "text" {
		t.Errorf("Expected default format 'text', got %v", config.Format)
	}

	if config.Output != "file" {
		t.Errorf("Expected default output 'file', got %v", config.Output)
	}

	if config.MaxSize != 10 {
		t.Errorf("Expected default MaxSize 10, got %v", config.MaxSize)
	}

	if config.MaxBackups != 5 {
		t.Errorf("Expected default MaxBackups 5, got %v", config.MaxBackups)
	}

	// Check log file path is set
	if config.LogFile == "" {
		t.Error("Expected default log file path to be set")
	}
}

func TestLogLevels(t *testing.T) {
	levels := []LogLevel{
		TraceLevel,
		DebugLevel,
		InfoLevel,
		WarnLevel,
		ErrorLevel,
		FatalLevel,
		PanicLevel,
	}

	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			// Test that level can be parsed
			_, err := logrus.ParseLevel(string(level))
			if err != nil {
				t.Errorf("Failed to parse log level %v: %v", level, err)
			}
		})
	}
}

func TestInitWithConsoleOutput(t *testing.T) {
	// Reset global logger
	globalLogger = nil
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	config := Config{
		Level:  DebugLevel,
		Format: "text",
		Output: "console",
	}

	err := Init(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	logger := GetLogger()
	if logger == nil {
		t.Fatal("GetLogger returned nil")
	}

	if logger.GetLevel() != logrus.DebugLevel {
		t.Errorf("Expected debug level, got %v", logger.GetLevel())
	}
}

func TestInitWithJSONFormat(t *testing.T) {
	// Reset global logger
	globalLogger = nil
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	config := Config{
		Level:  InfoLevel,
		Format: "json",
		Output: "console",
	}

	err := Init(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	logger := GetLogger()
	if logger == nil {
		t.Fatal("GetLogger returned nil")
	}

	// Check that formatter is JSONFormatter
	if _, ok := logger.Formatter.(*logrus.JSONFormatter); !ok {
		t.Error("Expected JSONFormatter")
	}
}

func TestInitWithFileOutput(t *testing.T) {
	// Reset global logger
	globalLogger = nil
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "aish-log-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test.log")

	config := Config{
		Level:   DebugLevel,
		Format:  "text",
		Output:  "file",
		LogFile: logFilePath,
	}

	err = Init(config)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer Close()

	// Check that log file was created
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}

	// Write a log and verify it's in the file
	logger := GetLogger()
	logger.Info("Test log message")

	// Close and read file
	Close()

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Test log message") {
		t.Error("Log message not found in file")
	}
}

func TestInitWithInvalidLevel(t *testing.T) {
	config := Config{
		Level:  LogLevel("invalid"),
		Format: "text",
		Output: "console",
	}

	err := Init(config)
	if err == nil {
		t.Error("Expected error for invalid log level")
	}

	if !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("Expected 'invalid log level' error, got: %v", err)
	}
}

func TestInitWithInvalidFormat(t *testing.T) {
	config := Config{
		Level:  InfoLevel,
		Format: "invalid",
		Output: "console",
	}

	err := Init(config)
	if err == nil {
		t.Error("Expected error for invalid log format")
	}

	if !strings.Contains(err.Error(), "invalid log format") {
		t.Errorf("Expected 'invalid log format' error, got: %v", err)
	}
}

func TestInitWithInvalidOutput(t *testing.T) {
	config := Config{
		Level:  InfoLevel,
		Format: "text",
		Output: "invalid",
	}

	err := Init(config)
	if err == nil {
		t.Error("Expected error for invalid log output")
	}

	if !strings.Contains(err.Error(), "invalid log output") {
		t.Errorf("Expected 'invalid log output' error, got: %v", err)
	}
}

func TestGetLogger(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	logger := GetLogger()
	if logger == nil {
		t.Fatal("GetLogger returned nil")
	}

	// Should return same instance on second call
	logger2 := GetLogger()
	if logger != logger2 {
		t.Error("GetLogger should return same instance")
	}
}

func TestWithComponent(t *testing.T) {
	// Initialize with console output for testing
	globalLogger = nil
	config := Config{
		Level:  DebugLevel,
		Format: "text",
		Output: "console",
	}
	Init(config)

	logger := WithComponent("test-component")
	if logger == nil {
		t.Fatal("WithComponent returned nil")
	}

	if logger.component != "test-component" {
		t.Errorf("Expected component 'test-component', got %v", logger.component)
	}
}

func TestLoggerWithField(t *testing.T) {
	// Initialize with console output
	globalLogger = nil
	var buf bytes.Buffer
	
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	globalLogger = &Logger{
		Logger:    logger,
		component: "test",
	}

	GetLogger().WithField("key", "value").Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected field 'key=value' in output, got: %s", output)
	}

	if !strings.Contains(output, "test message") {
		t.Errorf("Expected message in output, got: %s", output)
	}
}

func TestLoggerWithFields(t *testing.T) {
	// Initialize with console output
	globalLogger = nil
	var buf bytes.Buffer
	
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	globalLogger = &Logger{
		Logger:    logger,
		component: "test",
	}

	GetLogger().WithFields(logrus.Fields{
		"key1": "value1",
		"key2": "value2",
	}).Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key1=value1") || !strings.Contains(output, "key2=value2") {
		t.Errorf("Expected fields in output, got: %s", output)
	}
}

func TestLoggerWithError(t *testing.T) {
	// Initialize with console output
	globalLogger = nil
	var buf bytes.Buffer
	
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	globalLogger = &Logger{
		Logger:    logger,
		component: "test",
	}

	testErr := fmt.Errorf("test error")
	GetLogger().WithError(testErr).Error("error occurred")

	output := buf.String()
	if !strings.Contains(output, "test error") {
		t.Errorf("Expected error in output, got: %s", output)
	}
}

func TestSetAndGetLevel(t *testing.T) {
	// Initialize logger
	globalLogger = nil
	config := Config{
		Level:  InfoLevel,
		Format: "text",
		Output: "console",
	}
	Init(config)

	// Test SetLevel
	err := SetLevel(DebugLevel)
	if err != nil {
		t.Errorf("SetLevel failed: %v", err)
	}

	// Test GetLevel
	level := GetLevel()
	if level != DebugLevel {
		t.Errorf("Expected debug level, got %v", level)
	}

	// Test invalid level
	err = SetLevel(LogLevel("invalid"))
	if err == nil {
		t.Error("Expected error for invalid level")
	}
}

func TestGlobalConvenienceFunctions(t *testing.T) {
	// Initialize with console output to buffer
	globalLogger = nil
	var buf bytes.Buffer
	
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.TraceLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	globalLogger = &Logger{
		Logger:    logger,
		component: "test",
	}

	// Test each level (except Fatal and Panic which would exit/panic)
	Trace("trace message")
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	output := buf.String()
	
	if !strings.Contains(output, "trace message") {
		t.Error("Trace message not found")
	}
	if !strings.Contains(output, "debug message") {
		t.Error("Debug message not found")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Info message not found")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message not found")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message not found")
	}
}

func TestGlobalFormattedFunctions(t *testing.T) {
	// Initialize with console output to buffer
	globalLogger = nil
	var buf bytes.Buffer
	
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.TraceLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	globalLogger = &Logger{
		Logger:    logger,
		component: "test",
	}

	// Test formatted functions
	Tracef("trace %s", "formatted")
	Debugf("debug %s", "formatted")
	Infof("info %s", "formatted")
	Warnf("warn %s", "formatted")
	Errorf("error %s", "formatted")

	output := buf.String()
	
	if !strings.Contains(output, "trace formatted") {
		t.Error("Tracef message not found")
	}
	if !strings.Contains(output, "debug formatted") {
		t.Error("Debugf message not found")
	}
	if !strings.Contains(output, "info formatted") {
		t.Error("Infof message not found")
	}
	if !strings.Contains(output, "warn formatted") {
		t.Error("Warnf message not found")
	}
	if !strings.Contains(output, "error formatted") {
		t.Error("Errorf message not found")
	}
}

func TestCustomTextFormatter(t *testing.T) {
	formatter := &CustomTextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	}

	entry := &logrus.Entry{
		Logger:  logrus.New(),
		Level:   logrus.InfoLevel,
		Message: "test message",
		Data: logrus.Fields{
			"component": "test-component",
			"key":       "value",
		},
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	outputStr := string(output)

	// Check timestamp
	if !strings.Contains(outputStr, "-") {
		t.Error("Expected timestamp in output")
	}

	// Check log level
	if !strings.Contains(outputStr, "[INFO]") {
		t.Error("Expected [INFO] in output")
	}

	// Check component
	if !strings.Contains(outputStr, "[test-component]") {
		t.Error("Expected [test-component] in output")
	}

	// Check message
	if !strings.Contains(outputStr, "test message") {
		t.Error("Expected 'test message' in output")
	}

	// Check additional fields
	if !strings.Contains(outputStr, "key=value") {
		t.Error("Expected 'key=value' in output")
	}
}

func TestCustomTextFormatterWithoutTimestamp(t *testing.T) {
	formatter := &CustomTextFormatter{
		FullTimestamp: false,
	}

	entry := &logrus.Entry{
		Logger:  logrus.New(),
		Level:   logrus.WarnLevel,
		Message: "warning message",
		Data:    logrus.Fields{},
	}

	output, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	outputStr := string(output)

	// Should start with log level (no timestamp)
	if !strings.HasPrefix(outputStr, "[WARN]") && !strings.HasPrefix(outputStr, "[WARNING]") {
		t.Errorf("Expected output to start with [WARN], got: %s", outputStr)
	}
}

func TestCloseWithoutInit(t *testing.T) {
	// Reset state
	logFile = nil

	// Should not panic or error
	err := Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestCloseAfterFileInit(t *testing.T) {
	// Reset global logger
	globalLogger = nil
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	tmpDir, err := os.MkdirTemp("", "aish-close-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config := Config{
		Level:   InfoLevel,
		Format:  "text",
		Output:  "file",
		LogFile: filepath.Join(tmpDir, "test.log"),
	}

	err = Init(config)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Write something
	Info("test before close")

	// Close
	err = Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify logFile is closed
	if logFile != nil {
		// Try to write (should fail if properly closed)
		_, err := logFile.WriteString("test")
		if err == nil {
			t.Error("Expected error writing to closed file")
		}
	}
}

func TestLoggerMethods(t *testing.T) {
	// Initialize with buffer
	globalLogger = nil
	var buf bytes.Buffer
	
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetLevel(logrus.TraceLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	testLogger := &Logger{
		Logger:    logger,
		component: "test-methods",
	}

	// Test all convenience methods
	testLogger.Trace("trace")
	testLogger.Debug("debug")
	testLogger.Info("info")
	testLogger.Warn("warn")
	testLogger.Error("error")

	testLogger.Tracef("trace %d", 1)
	testLogger.Debugf("debug %d", 2)
	testLogger.Infof("info %d", 3)
	testLogger.Warnf("warn %d", 4)
	testLogger.Errorf("error %d", 5)

	output := buf.String()

	expectedStrings := []string{
		"trace", "debug", "info", "warn", "error",
		"trace 1", "debug 2", "info 3", "warn 4", "error 5",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected '%s' in output, got: %s", expected, output)
		}
	}
}

func TestGlobalWithFunctions(t *testing.T) {
	// Initialize with buffer
	globalLogger = nil
	var buf bytes.Buffer
	
	logger := logrus.New()
	logger.SetOutput(&buf)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	globalLogger = &Logger{
		Logger:    logger,
		component: "global-test",
	}

	// Test WithField
	WithField("testkey", "testvalue").Info("field test")

	// Test WithFields
	WithFields(logrus.Fields{
		"key1": "val1",
		"key2": "val2",
	}).Info("fields test")

	// Test WithError
	testErr := fmt.Errorf("test error")
	WithError(testErr).Error("error test")

	output := buf.String()

	if !strings.Contains(output, "testkey=testvalue") {
		t.Error("WithField output not found")
	}

	if !strings.Contains(output, "key1=val1") || !strings.Contains(output, "key2=val2") {
		t.Error("WithFields output not found")
	}

	if !strings.Contains(output, "test error") {
		t.Error("WithError output not found")
	}
}
