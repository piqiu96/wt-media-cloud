// Package logger creates generic Hertz-compatible log resources.
// It owns no application logger names or business semantics.
package logger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	hertzzap "github.com/hertz-contrib/logger/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config defines a normal log file, its threshold, encoding, and rotation.
type Config struct {
	Path     string
	Level    string
	Format   string
	Rotation RotationConfig
}

// RotationConfig is shared by a normal file and its warning/fatal companion.
type RotationConfig struct {
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Compress   bool
	LocalTime  bool
}

// Logger is a Hertz FullLogger backed by two Zap cores.
type Logger struct {
	*hertzzap.Logger
	zapLogger *zap.Logger

	main *lumberjack.Logger
	wf   *lumberjack.Logger

	closeOnce sync.Once
	closeErr  error
}

// New validates cfg, opens the target paths, and creates normal plus `.wf` cores.
func New(cfg Config) (*Logger, error) {
	if strings.TrimSpace(cfg.Path) == "" {
		return nil, errors.New("logger path is required")
	}
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	format, err := parseFormat(cfg.Format)
	if err != nil {
		return nil, err
	}
	if err := ensureLogPath(cfg.Path); err != nil {
		return nil, err
	}
	if err := ensureLogPath(cfg.Path + ".wf"); err != nil {
		return nil, err
	}

	main := newWriter(cfg.Path, cfg.Rotation)
	wf := newWriter(cfg.Path+".wf", cfg.Rotation)
	hertzLogger := hertzzap.NewLogger(
		hertzzap.WithCores(
			hertzzap.CoreConfig{
				Enc: newEncoder(format),
				Ws:  zapcore.AddSync(main),
				Lvl: zap.NewAtomicLevelAt(level),
			},
			hertzzap.CoreConfig{
				Enc: newEncoder(format),
				Ws:  zapcore.AddSync(wf),
				Lvl: zap.NewAtomicLevelAt(zap.WarnLevel),
			},
		),
		hertzzap.WithExtraKeys([]hertzzap.ExtraKey{
			"trace_id",
			"request_id",
			"user_id",
		}),
		hertzzap.WithExtraKeyAsStr(),
		hertzzap.WithZapOptions(zap.AddCaller()),
	)

	return &Logger{Logger: hertzLogger, zapLogger: hertzLogger.Logger(), main: main, wf: wf}, nil
}

// CtxTracef writes a trace-level message with non-empty generic context fields.
func (l *Logger) CtxTracef(ctx context.Context, format string, args ...any) {
	l.logContext(ctx, zap.DebugLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) CtxDebugf(ctx context.Context, format string, args ...any) {
	l.logContext(ctx, zap.DebugLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) CtxInfof(ctx context.Context, format string, args ...any) {
	l.logContext(ctx, zap.InfoLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) CtxNoticef(ctx context.Context, format string, args ...any) {
	l.logContext(ctx, zap.WarnLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) CtxWarnf(ctx context.Context, format string, args ...any) {
	l.logContext(ctx, zap.WarnLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) CtxErrorf(ctx context.Context, format string, args ...any) {
	l.logContext(ctx, zap.ErrorLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) CtxFatalf(ctx context.Context, format string, args ...any) {
	l.logContext(ctx, zap.FatalLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) logContext(ctx context.Context, level zapcore.Level, message string) {
	fields := contextFields(ctx)
	if level == zap.FatalLevel {
		l.zapLogger.Fatal(message, fields...)
		return
	}
	l.zapLogger.Log(level, message, fields...)
}

func contextFields(ctx context.Context) []zap.Field {
	var fields []zap.Field
	if traceID, ok := ctx.Value("trace_id").(string); ok && traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if requestID, ok := ctx.Value("request_id").(string); ok && requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if userID, ok := ctx.Value("user_id").(int64); ok && userID != 0 {
		fields = append(fields, zap.Int64("user_id", userID))
	}
	return fields
}

// Close syncs and closes both writers and is safe to call repeatedly.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.closeOnce.Do(func() {
		l.Logger.Sync()
		l.closeErr = errors.Join(l.main.Close(), l.wf.Close())
	})
	return l.closeErr
}

func newWriter(path string, rotation RotationConfig) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    rotation.MaxSize,
		MaxAge:     rotation.MaxAge,
		MaxBackups: rotation.MaxBackups,
		Compress:   rotation.Compress,
		LocalTime:  rotation.LocalTime,
	}
}

func ensureLogPath(path string) error {
	directory := filepath.Dir(path)
	if directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create logger directory: %w", err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open logger path: %w", err)
	}
	return file.Close()
}

func parseLevel(value string) (zapcore.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return zap.DebugLevel, nil
	case "info":
		return zap.InfoLevel, nil
	case "warn", "warning":
		return zap.WarnLevel, nil
	case "error":
		return zap.ErrorLevel, nil
	case "fatal":
		return zap.FatalLevel, nil
	default:
		return zap.InfoLevel, fmt.Errorf("invalid logger level %q", value)
	}
}

func parseFormat(value string) (string, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case "json":
		return "json", nil
	case "console", "text":
		return "console", nil
	default:
		return "", fmt.Errorf("invalid logger format %q", value)
	}
}

func newEncoder(format string) zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "time"
	cfg.MessageKey = "msg"
	cfg.LevelKey = "level"
	cfg.NameKey = "logger"
	cfg.CallerKey = "caller"
	cfg.StacktraceKey = "stacktrace"
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	cfg.EncodeDuration = zapcore.StringDurationEncoder
	cfg.EncodeCaller = zapcore.ShortCallerEncoder

	if format == "console" {
		return zapcore.NewConsoleEncoder(cfg)
	}
	return zapcore.NewJSONEncoder(cfg)
}
