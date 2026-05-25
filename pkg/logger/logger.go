//go:build linux

// Package logger 提供结构化日志，支持 TraceID 和分级日志
package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/trace"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String 返回日志级别字符串
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel 解析日志级别
func ParseLevel(s string) LogLevel {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// Fields 日志字段
type Fields map[string]interface{}

// Logger 日志接口
type Logger struct {
	level      LogLevel
	output     io.Writer
	formatter  Formatter
	fields     Fields
	fieldsMu   sync.RWMutex
}

// Formatter 格式化接口
type Formatter interface {
	Format(level LogLevel, msg string, fields Fields) ([]byte, error)
}

// JSONFormatter JSON 格式
type JSONFormatter struct {
	TimestampFormat string
}

// Format 格式化日志为 JSON
func (f *JSONFormatter) Format(level LogLevel, msg string, fields Fields) ([]byte, error) {
	data := make(map[string]interface{}, len(fields)+3)

	// 基础字段
	data["timestamp"] = time.Now().Format(f.TimestampFormat)
	data["level"] = level.String()
	data["message"] = msg

	// 自定义字段
	for k, v := range fields {
		data[k] = v
	}

	return json.Marshal(data)
}

// TextFormatter 文本格式
type TextFormatter struct {
	TimestampFormat string
}

// Format 格式化日志为文本
func (f *TextFormatter) Format(level LogLevel, msg string, fields Fields) ([]byte, error) {
	var sb strings.Builder

	// 时间戳
	sb.WriteString(time.Now().Format(f.TimestampFormat))
	sb.WriteString(" ")

	// 级别
	sb.WriteString("[")
	sb.WriteString(level.String())
	sb.WriteString("] ")

	// 消息
	sb.WriteString(msg)

	// 字段
	if len(fields) > 0 {
		sb.WriteString(" ")
		first := true
		for k, v := range fields {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprintf("%v", v))
			first = false
		}
	}

	sb.WriteString("\n")
	return []byte(sb.String()), nil
}

var (
	defaultLogger *Logger
	defaultOnce   sync.Once
)

// Default 获取默认日志实例
func Default() *Logger {
	defaultOnce.Do(func() {
		defaultLogger = NewLogger(INFO, "json")
	})
	return defaultLogger
}

// SetDefault 设置默认日志实例
func SetDefault(l *Logger) {
	defaultLogger = l
}

// NewLogger 创建日志实例
func NewLogger(level LogLevel, format string) *Logger {
	var formatter Formatter
	timestampFormat := "2006-01-02T15:04:05.000Z07:00"

	switch format {
	case "json":
		formatter = &JSONFormatter{TimestampFormat: timestampFormat}
	case "text":
		formatter = &TextFormatter{TimestampFormat: timestampFormat}
	default:
		formatter = &JSONFormatter{TimestampFormat: timestampFormat}
	}

	return &Logger{
		level:     level,
		output:    os.Stdout,
		formatter: formatter,
		fields:    make(Fields),
	}
}

// NewLoggerWithWriter 创建带自定义输出器的日志实例
func NewLoggerWithWriter(level LogLevel, format string, output io.Writer) *Logger {
	l := NewLogger(level, format)
	l.output = output
	return l
}

// WithField 添加字段
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		level:     l.level,
		output:    l.output,
		formatter: l.formatter,
		fields:    make(Fields),
	}

	// 复制原有字段
	l.fieldsMu.RLock()
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	l.fieldsMu.RUnlock()

	// 添加新字段
	newLogger.fields[key] = value
	return newLogger
}

// WithFields 添加多个字段
func (l *Logger) WithFields(fields Fields) *Logger {
	newLogger := &Logger{
		level:     l.level,
		output:    l.output,
		formatter: l.formatter,
		fields:    make(Fields),
	}

	// 复制原有字段
	l.fieldsMu.RLock()
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	l.fieldsMu.RUnlock()

	// 添加新字段
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithContext 从 context 创建带 TraceID 的日志
func (l *Logger) WithContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}

	traceID := trace.FromContext(ctx)
	if traceID == "" {
		return l
	}

	return l.WithField("trace_id", traceID)
}

// WithTraceID 创建带 TraceID 的日志
func (l *Logger) WithTraceID(traceID string) *Logger {
	return l.WithField("trace_id", traceID)
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel 获取日志级别
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// log 内部日志方法
func (l *Logger) log(level LogLevel, msg string, fields Fields) {
	if level < l.level {
		return
	}

	// 合并字段
	allFields := make(Fields)
	l.fieldsMu.RLock()
	for k, v := range l.fields {
		allFields[k] = v
	}
	l.fieldsMu.RUnlock()

	for k, v := range fields {
		allFields[k] = v
	}

	// 格式化
	data, err := l.formatter.Format(level, msg, allFields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to format log: %v\n", err)
		return
	}

	// 输出
	l.output.Write(data)

	// FATAL 级别退出程序
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug 调试日志
func (l *Logger) Debug(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, msg, f)
}

// Info 信息日志
func (l *Logger) Info(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, msg, f)
}

// Warn 警告日志
func (l *Logger) Warn(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, msg, f)
}

// Error 错误日志
func (l *Logger) Error(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, msg, f)
}

// Fatal 致命日志
func (l *Logger) Fatal(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, msg, f)
}

// Debugf 格式化调试日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// Infof 格式化信息日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warnf 格式化警告日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Errorf 格式化错误日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// Fatalf 格式化致命日志
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Fatal(fmt.Sprintf(format, args...))
}

// ============================================================
// 全局快捷方法
// ============================================================

// WithContext 从 context 获取带 TraceID 的日志
func WithContext(ctx context.Context) *Logger {
	return Default().WithContext(ctx)
}

// WithTraceID 获取带 TraceID 的日志
func WithTraceID(traceID string) *Logger {
	return Default().WithTraceID(traceID)
}

// Debug 全局调试日志
func Debug(msg string, fields ...Fields) {
	Default().Debug(msg, fields...)
}

// Info 全局信息日志
func Info(msg string, fields ...Fields) {
	Default().Info(msg, fields...)
}

// Warn 全局警告日志
func Warn(msg string, fields ...Fields) {
	Default().Warn(msg, fields...)
}

// Error 全局错误日志
func Error(msg string, fields ...Fields) {
	Default().Error(msg, fields...)
}

// Fatal 全局致命日志
func Fatal(msg string, fields ...Fields) {
	Default().Fatal(msg, fields...)
}

// Debugf 全局格式化调试日志
func Debugf(format string, args ...interface{}) {
	Default().Debugf(format, args...)
}

// Infof 全局格式化信息日志
func Infof(format string, args ...interface{}) {
	Default().Infof(format, args...)
}

// Warnf 全局格式化警告日志
func Warnf(format string, args ...interface{}) {
	Default().Warnf(format, args...)
}

// Errorf 全局格式化错误日志
func Errorf(format string, args ...interface{}) {
	Default().Errorf(format, args...)
}

// Fatalf 全局格式化致命日志
func Fatalf(format string, args ...interface{}) {
	Default().Fatalf(format, args...)
}
