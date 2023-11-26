package logger

import (
	"github.com/fatih/color"
	"go.uber.org/zap/zapcore"
)

func colorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	colorLevel := color.New()

	switch l {
	case zapcore.DebugLevel:
		colorLevel.Add(color.FgCyan)
	case zapcore.InfoLevel:
		colorLevel.Add(color.FgGreen)
	case zapcore.WarnLevel:
		colorLevel.Add(color.FgYellow)
	case zapcore.ErrorLevel, zapcore.DPanicLevel:
		colorLevel.Add(color.FgHiRed)
	case zapcore.PanicLevel, zapcore.FatalLevel:
		colorLevel.Add(color.FgRed)
	default:
		colorLevel.Add(color.Reset)
	}

	enc.AppendString(colorLevel.Sprint(l.CapitalString()))
}
