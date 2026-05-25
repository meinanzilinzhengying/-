// Package logger 提供日志记录功能
package logger

import (
	"fmt"
	"log"
)

// Logger 日志记录器
type Logger struct {
	prefix string
}

// New 创建新的日志记录器
func New(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

// Infof 记录信息级别日志
func (l *Logger) Infof(format string, args ...interface{}) {
	log.Printf("[INFO] %s: %s", l.prefix, fmt.Sprintf(format, args...))
}

// Warnf 记录警告级别日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	log.Printf("[WARN] %s: %s", l.prefix, fmt.Sprintf(format, args...))
}

// Debugf 记录调试级别日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	log.Printf("[DEBUG] %s: %s", l.prefix, fmt.Sprintf(format, args...))
}

// Errorf 记录错误级别日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	log.Printf("[ERROR] %s: %s", l.prefix, fmt.Sprintf(format, args...))
}

// Info 记录信息级别日志
func (l *Logger) Info(args ...interface{}) {
	log.Printf("[INFO] %s: %s", l.prefix, fmt.Sprint(args...))
}
