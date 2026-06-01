package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	once   sync.Once
	global *zap.Logger
)

func Init(level, format, output, file string) {
	once.Do(func() {
		global = newLogger(level, format, output, file)
	})
}

func newLogger(level, format, output, file string) *zap.Logger {
	var zapLevel zapcore.Level
	_ = zapLevel.UnmarshalText([]byte(level))

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	var writers []zapcore.WriteSyncer
	if output == "stdout" || output == "" {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	}
	if file != "" {
		// 确保日志目录存在（Windows/Linux/macOS 通用）
		if dirErr := os.MkdirAll(filepath.Dir(file), 0755); dirErr != nil {
			fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", dirErr)
		}
		f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			writers = append(writers, zapcore.AddSync(f))
		} else {
			fmt.Fprintf(os.Stderr, "打开日志文件失败: %v，将只输出到 stdout\n", err)
		}
	}

	core := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(writers...), zapLevel)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

func L() *zap.Logger {
	if global == nil {
		global = newLogger("info", "json", "stdout", "")
	}
	return global
}

func Info(msg string, fields ...zap.Field)  { L().Info(msg, fields...) }
func Error(msg string, fields ...zap.Field) { L().Error(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { L().Warn(msg, fields...) }
func Debug(msg string, fields ...zap.Field) { L().Debug(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) { L().Fatal(msg, fields...) }
