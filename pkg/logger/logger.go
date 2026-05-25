// Package logger 提供结构化日志工具
package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	// DEBUG 调试级别
	DEBUG LogLevel = iota
	// INFO 信息级别
	INFO
	// WARN 警告级别
	WARN
	// ERROR 错误级别
	ERROR
	// FATAL 致命级别
	FATAL
)

// String 返回日志级别的字符串表示
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

// ParseLevel 从字符串解析日志级别
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

// Field 日志字段
type Field struct {
	Key   string
	Value interface{}
}

// F 创建日志字段的便捷函数
func F(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Fields 日志字段集合
type Fields map[string]interface{}

// Logger 日志接口
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	WithFields(fields ...Field) Logger
	WithContext(ctx context.Context) Logger
	WithError(err error) Logger

	SetLevel(level LogLevel)
	GetLevel() LogLevel
}

// Config 日志配置
type Config struct {
	Level          LogLevel
	Output         io.Writer
	Format         string // json, text
	TimeFormat     string
	EnableCaller   bool
	CallerSkip     int
	EnableColor    bool
	FileOutput     string
	MaxSize        int // MB
	MaxBackups     int
	MaxAge         int // days
	Compress       bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:        INFO,
		Output:       os.Stdout,
		Format:       "text",
		TimeFormat:   "2006-01-02 15:04:05.000",
		EnableCaller: true,
		CallerSkip:   2,
		EnableColor:  true,
	}
}

// stdLogger 标准日志实现
type stdLogger struct {
	level        LogLevel
	output       io.Writer
	format       string
	timeFormat   string
	enableCaller bool
	callerSkip   int
	enableColor  bool
	fields       Fields
	mu           sync.RWMutex
}

// New 创建新的日志实例
func New(config *Config) Logger {
	if config == nil {
		config = DefaultConfig()
	}

	return &stdLogger{
		level:        config.Level,
		output:       config.Output,
		format:       config.Format,
		timeFormat:   config.TimeFormat,
		enableCaller: config.EnableCaller,
		callerSkip:   config.CallerSkip,
		enableColor:  config.EnableColor,
		fields:       make(Fields),
	}
}

// NewLogger 创建新的日志实例（便捷函数）
func NewLogger(level LogLevel) Logger {
	return New(&Config{
		Level:        level,
		Output:       os.Stdout,
		Format:       "text",
		TimeFormat:   "2006-01-02 15:04:05.000",
		EnableCaller: true,
		CallerSkip:   2,
		EnableColor:  true,
	})
}

// defaultLogger 默认日志实例
var defaultLogger = New(DefaultConfig())

// ==================== 日志方法 ====================

func (l *stdLogger) Debug(msg string, fields ...Field) {
	l.log(DEBUG, msg, fields...)
}

func (l *stdLogger) Info(msg string, fields ...Field) {
	l.log(INFO, msg, fields...)
}

func (l *stdLogger) Warn(msg string, fields ...Field) {
	l.log(WARN, msg, fields...)
}

func (l *stdLogger) Error(msg string, fields ...Field) {
	l.log(ERROR, msg, fields...)
}

func (l *stdLogger) Fatal(msg string, fields ...Field) {
	l.log(FATAL, msg, fields...)
	os.Exit(1)
}

func (l *stdLogger) WithFields(fields ...Field) Logger {
	newFields := make(Fields)
	for k, v := range l.fields {
		newFields[k] = v
	}
	for _, f := range fields {
		newFields[f.Key] = f.Value
	}

	return &stdLogger{
		level:        l.level,
		output:       l.output,
		format:       l.format,
		timeFormat:   l.timeFormat,
		enableCaller: l.enableCaller,
		callerSkip:   l.callerSkip,
		enableColor:  l.enableColor,
		fields:       newFields,
	}
}

func (l *stdLogger) WithContext(ctx context.Context) Logger {
	// 从上下文中提取trace_id等字段
	fields := make([]Field, 0)
	if traceID := ctx.Value("trace_id"); traceID != nil {
		fields = append(fields, F("trace_id", traceID))
	}
	if spanID := ctx.Value("span_id"); spanID != nil {
		fields = append(fields, F("span_id", spanID))
	}
	return l.WithFields(fields...)
}

func (l *stdLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}
	return l.WithFields(F("error", err.Error()))
}

func (l *stdLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *stdLogger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// log 核心日志方法
func (l *stdLogger) log(level LogLevel, msg string, fields ...Field) {
	if level < l.GetLevel() {
		return
	}

	entry := &logEntry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Fields:  make(Fields),
	}

	// 复制默认字段
	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	// 添加传入的字段
	for _, f := range fields {
		entry.Fields[f.Key] = f.Value
	}

	// 添加调用者信息
	if l.enableCaller {
		entry.Caller = getCaller(l.callerSkip + 1)
	}

	// 格式化并输出
	output := l.formatEntry(entry)
	l.mu.Lock()
	fmt.Fprintln(l.output, output)
	l.mu.Unlock()
}

// logEntry 日志条目
type logEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
	Fields  Fields
	Caller  string
}

// formatEntry 格式化日志条目
func (l *stdLogger) formatEntry(entry *logEntry) string {
	if l.format == "json" {
		return l.formatJSON(entry)
	}
	return l.formatText(entry)
}

// formatText 文本格式
func (l *stdLogger) formatText(entry *logEntry) string {
	var sb strings.Builder

	// 时间
	sb.WriteString(entry.Time.Format(l.timeFormat))
	sb.WriteString(" ")

	// 级别
	levelStr := entry.Level.String()
	if l.enableColor {
		levelStr = colorizeLevel(entry.Level)
	}
	sb.WriteString("[")
	sb.WriteString(levelStr)
	sb.WriteString("] ")

	// 调用者
	if l.enableCaller && entry.Caller != "" {
		sb.WriteString(entry.Caller)
		sb.WriteString(" ")
	}

	// 消息
	sb.WriteString(entry.Message)

	// 字段
	if len(entry.Fields) > 0 {
		sb.WriteString(" ")
		first := true
		for k, v := range entry.Fields {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprintf("%v", v))
			first = false
		}
	}

	return sb.String()
}

// formatJSON JSON格式
func (l *stdLogger) formatJSON(entry *logEntry) string {
	data := make(map[string]interface{})
	data["time"] = entry.Time.Format(l.timeFormat)
	data["level"] = entry.Level.String()
	data["message"] = entry.Message
	if l.enableCaller && entry.Caller != "" {
		data["caller"] = entry.Caller
	}
	for k, v := range entry.Fields {
		data[k] = v
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal log entry: %v"}`, err)
	}
	return string(jsonData)
}

// getCaller 获取调用者信息
func getCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown"
	}
	// 简化路径
	file = filepath.Base(file)
	return fmt.Sprintf("%s:%d", file, line)
}

// colorizeLevel 为日志级别添加颜色
func colorizeLevel(level LogLevel) string {
	const (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorGreen  = "\033[32m"
		colorYellow = "\033[33m"
		colorBlue   = "\033[34m"
		colorPurple = "\033[35m"
	)

	switch level {
	case DEBUG:
		return colorBlue + "DEBUG" + colorReset
	case INFO:
		return colorGreen + "INFO" + colorReset
	case WARN:
		return colorYellow + "WARN" + colorReset
	case ERROR:
		return colorRed + "ERROR" + colorReset
	case FATAL:
		return colorPurple + "FATAL" + colorReset
	default:
		return level.String()
	}
}

// ==================== 全局便捷函数 ====================

// Debug 输出调试日志
func Debug(msg string, fields ...Field) {
	defaultLogger.Debug(msg, fields...)
}

// Info 输出信息日志
func Info(msg string, fields ...Field) {
	defaultLogger.Info(msg, fields...)
}

// Warn 输出警告日志
func Warn(msg string, fields ...Field) {
	defaultLogger.Warn(msg, fields...)
}

// Error 输出错误日志
func Error(msg string, fields ...Field) {
	defaultLogger.Error(msg, fields...)
}

// Fatal 输出致命日志并退出
func Fatal(msg string, fields ...Field) {
	defaultLogger.Fatal(msg, fields...)
}

// Debugf 格式化输出调试日志
func Debugf(format string, args ...interface{}) {
	defaultLogger.Debug(fmt.Sprintf(format, args...))
}

// Infof 格式化输出信息日志
func Infof(format string, args ...interface{}) {
	defaultLogger.Info(fmt.Sprintf(format, args...))
}

// Warnf 格式化输出警告日志
func Warnf(format string, args ...interface{}) {
	defaultLogger.Warn(fmt.Sprintf(format, args...))
}

// Errorf 格式化输出错误日志
func Errorf(format string, args ...interface{}) {
	defaultLogger.Error(fmt.Sprintf(format, args...))
}

// Fatalf 格式化输出致命日志并退出
func Fatalf(format string, args ...interface{}) {
	defaultLogger.Fatal(fmt.Sprintf(format, args...))
}

// WithFields 添加字段
func WithFields(fields ...Field) Logger {
	return defaultLogger.WithFields(fields...)
}

// WithContext 添加上下文
func WithContext(ctx context.Context) Logger {
	return defaultLogger.WithContext(ctx)
}

// WithError 添加错误
func WithError(err error) Logger {
	return defaultLogger.WithError(err)
}

// SetLevel 设置日志级别
func SetLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// GetLevel 获取日志级别
func GetLevel() LogLevel {
	return defaultLogger.GetLevel()
}

// SetOutput 设置输出
func SetOutput(output io.Writer) {
	if l, ok := defaultLogger.(*stdLogger); ok {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.output = output
	}
}

// ==================== 文件日志 ====================

// FileLogger 文件日志
type FileLogger struct {
	filename   string
	maxSize    int
	maxBackups int
	maxAge     int
	compress   bool
	file       *os.File
	size       int64
	mu         sync.Mutex
}

// NewFileLogger 创建文件日志
func NewFileLogger(filename string, maxSize, maxBackups, maxAge int, compress bool) (*FileLogger, error) {
	fl := &FileLogger{
		filename:   filename,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		maxAge:     maxAge,
		compress:   compress,
	}

	if err := fl.open(); err != nil {
		return nil, err
	}

	return fl, nil
}

func (fl *FileLogger) open() error {
	// 创建目录
	dir := filepath.Dir(fl.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 打开文件
	file, err := os.OpenFile(fl.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	fl.file = file
	info, err := file.Stat()
	if err != nil {
		return err
	}
	fl.size = info.Size()

	return nil
}

// Write 实现io.Writer接口
func (fl *FileLogger) Write(p []byte) (n int, err error) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	// 检查是否需要轮转
	if fl.size+int64(len(p)) > int64(fl.maxSize*1024*1024) {
		if err := fl.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = fl.file.Write(p)
	fl.size += int64(n)
	return n, err
}

// rotate 日志轮转
func (fl *FileLogger) rotate() error {
	// 关闭当前文件
	if err := fl.file.Close(); err != nil {
		return err
	}

	// 重命名文件
	timestamp := time.Now().Format("20060102150405")
	backupName := fmt.Sprintf("%s.%s", fl.filename, timestamp)
	if err := os.Rename(fl.filename, backupName); err != nil {
		return err
	}

	// 重新打开文件
	return fl.open()
}

// Close 关闭文件
func (fl *FileLogger) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	return fl.file.Close()
}

// ==================== 辅助函数 ====================

// Init 初始化日志
func Init(config *Config) {
	defaultLogger = New(config)
}

// InitWithFile 初始化带文件输出的日志
func InitWithFile(level LogLevel, filename string) {
	fileLogger, err := NewFileLogger(filename, 100, 10, 30, true)
	if err != nil {
		log.Fatalf("Failed to create file logger: %v", err)
	}

	// 同时输出到控制台和文件
	multiWriter := io.MultiWriter(os.Stdout, fileLogger)

	Init(&Config{
		Level:        level,
		Output:       multiWriter,
		Format:       "text",
		TimeFormat:   "2006-01-02 15:04:05.000",
		EnableCaller: true,
		CallerSkip:   2,
		EnableColor:  true,
	})
}

// Sync 刷新日志缓冲区
func Sync() error {
	// 标准日志不需要刷新
	return nil
}
