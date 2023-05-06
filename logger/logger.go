package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var logger *zap.SugaredLogger

func init() {
	// First, define our level-handling logic.
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel
	})

	// Directly output to stdout and stderr, and add caller information.
	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleErrors := zapcore.Lock(os.Stderr)
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
	encoderConfig.ConsoleSeparator = " "
	encoderConfig.EncodeLevel = colorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Join the outputs, encoders, and level-handling functions into
	// zapcore.Cores, then tee the two cores together.
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, consoleErrors, highPriority),
		zapcore.NewCore(consoleEncoder, consoleDebugging, lowPriority),
	)

	// From a zapcore.Core, it's easy to construct a Logger.
	logger = zap.New(core, zap.AddCaller()).WithOptions(zap.AddCallerSkip(1)).Sugar()
}

func Sync() {
	_ = logger.Sync()
}

func GetLogger() *zap.SugaredLogger {
	return logger
}

func Info(args ...interface{}) {
	logger.Infoln(args...)
}

func Error(args ...interface{}) {
	logger.Errorln(args...)
}

func Fatal(args ...interface{}) {
	logger.Fatalln(args...)
}

func Warn(args ...interface{}) {
	logger.Warnln(args...)
}

func Debug(args ...interface{}) {
	logger.Debugln(args...)
}

func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
}

func Warnf(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}
