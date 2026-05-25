// Package logmgr 日志分级管理与轮转清理
// 分级输出(DEBUG/INFO/WARN/ERROR)，按大小/天数自动轮转
package logmgr

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 日志级别
// ============================================================

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String 日志级别字符串
func (l Level) String() string {
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

// Color ANSI颜色码
func (l Level) Color() string {
	switch l {
	case DEBUG:
		return "\033[36m" // cyan
	case INFO:
		return "\033[32m" // green
	case WARN:
		return "\033[33m" // yellow
	case ERROR:
		return "\033[31m" // red
	case FATAL:
		return "\033[35m" // magenta
	default:
		return "\033[0m"
	}
}

// Reset 重置颜色
func Reset() string {
	return "\033[0m"
}

// ParseLevel 解析日志级别
func ParseLevel(s string) Level {
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

// ============================================================
// 日志配置
// ============================================================

// Config 日志配置
type Config struct {
	Level            string `yaml:"level" json:"level"`                         // 最低日志级别
	OutputMode       string `yaml:"output_mode" json:"output_mode"`             // stdout/file/both
	LogDir           string `yaml:"log_dir" json:"log_dir"`                     // 日志目录
	LogFile          string `yaml:"log_file" json:"log_file"`                   // 日志文件名
	MaxFileSize      int64  `yaml:"max_file_size" json:"max_file_size"`         // 单文件最大大小 (MB)
	MaxTotalSize     int64  `yaml:"max_total_size" json:"max_total_size"`       // 总日志最大大小 (MB)
	MaxRetentionDays int    `yaml:"max_retention_days" json:"max_retention_days"` // 最大保留天数
	MaxFileCount     int    `yaml:"max_file_count" json:"max_file_count"`       // 最大文件数
	EnableColor      bool   `yaml:"enable_color" json:"enable_color"`           // 启用颜色
	EnableRotation   bool   `yaml:"enable_rotation" json:"enable_rotation"`     // 启用轮转
	RotationCheckSec int    `yaml:"rotation_check_sec" json:"rotation_check_sec"` // 轮转检查间隔（秒）
	EnableCompression bool  `yaml:"enable_compression" json:"enable_compression"` // 启用压缩
	Format           string `yaml:"format" json:"format"`                       // 日志格式: text/json
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:            "INFO",
		OutputMode:       "both",
		LogDir:           "/var/log/cloud-flow",
		LogFile:          "agent.log",
		MaxFileSize:      100,    // 100MB
		MaxTotalSize:     1024,   // 1GB
		MaxRetentionDays: 7,
		MaxFileCount:     10,
		EnableColor:      true,
		EnableRotation:   true,
		RotationCheckSec: 30,
		EnableCompression: false,
		Format:           "text",
	}
}

// ============================================================
// 日志管理器
// ============================================================

// Manager 日志管理器
type Manager struct {
	config   *Config
	minLevel Level
	
	// 输出
	fileWriter *RotateWriter
	consoleWriter io.Writer
	
	// 控制
	stopCh chan struct{}
	wg     sync.WaitGroup
	
	mu sync.Mutex
}

// NewManager 创建日志管理器
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	m := &Manager{
		config:   config,
		minLevel: ParseLevel(config.Level),
		consoleWriter: os.Stdout,
		stopCh:   make(chan struct{}),
	}
	
	// 初始化文件输出
	if config.OutputMode == "file" || config.OutputMode == "both" {
		if err := os.MkdirAll(config.LogDir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}
		
		logPath := filepath.Join(config.LogDir, config.LogFile)
		rw, err := NewRotateWriter(logPath, &RotateConfig{
			MaxFileSize:      config.MaxFileSize * 1024 * 1024,
			MaxFileCount:     config.MaxFileCount,
			MaxRetentionDays: config.MaxRetentionDays,
			MaxTotalSize:     config.MaxTotalSize * 1024 * 1024,
			EnableCompression: config.EnableCompression,
		})
		if err != nil {
			return nil, fmt.Errorf("创建日志文件失败: %w", err)
		}
		m.fileWriter = rw
	}
	
	// 启动轮转检查
	if config.EnableRotation {
		m.wg.Add(1)
		go m.rotationLoop()
	}
	
	return m, nil
}

// Log 记录日志
func (m *Manager) Log(level Level, format string, args ...interface{}) {
	if level < m.minLevel {
		return
	}
	
	now := time.Now()
	msg := fmt.Sprintf(format, args...)
	
	// 格式化日志行
	var line string
	if m.config.Format == "json" {
		line = m.formatJSON(now, level, msg)
	} else {
		line = m.formatText(now, level, msg)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 输出到控制台
	if m.config.OutputMode == "stdout" || m.config.OutputMode == "both" {
		consoleLine := line
		if m.config.EnableColor {
			consoleLine = fmt.Sprintf("%s%s%s %s%s", level.Color(), now.Format("2006-01-02 15:04:05"), level.String(), Reset(), msg, Reset())
		}
		fmt.Fprintln(m.consoleWriter, consoleLine)
	}
	
	// 输出到文件
	if m.fileWriter != nil {
		m.fileWriter.Write([]byte(line + "\n"))
	}
}

// Debug 记录DEBUG日志
func (m *Manager) Debug(format string, args ...interface{}) {
	m.Log(DEBUG, format, args...)
}

// Info 记录INFO日志
func (m *Manager) Info(format string, args ...interface{}) {
	m.Log(INFO, format, args...)
}

// Warn 记录WARN日志
func (m *Manager) Warn(format string, args ...interface{}) {
	m.Log(WARN, format, args...)
}

// Error 记录ERROR日志
func (m *Manager) Error(format string, args ...interface{}) {
	m.Log(ERROR, format, args...)
}

// Fatal 记录FATAL日志并退出
func (m *Manager) Fatal(format string, args ...interface{}) {
	m.Log(FATAL, format, args...)
	os.Exit(1)
}

// SetLevel 动态设置日志级别
func (m *Manager) SetLevel(level string) {
	m.minLevel = ParseLevel(level)
	m.config.Level = level
}

// GetLevel 获取当前日志级别
func (m *Manager) GetLevel() string {
	return m.minLevel.String()
}

// Close 关闭日志管理器
func (m *Manager) Close() {
	close(m.stopCh)
	m.wg.Wait()
	
	if m.fileWriter != nil {
		m.fileWriter.Close()
	}
}

// ============================================================
// 格式化
// ============================================================

func (m *Manager) formatText(t time.Time, level Level, msg string) string {
	// 2006-01-02 15:04:05 [INFO] message
	return fmt.Sprintf("%s [%s] %s", t.Format("2006-01-02 15:04:05"), level.String(), msg)
}

func (m *Manager) formatJSON(t time.Time, level Level, msg string) string {
	// 简化JSON格式
	escaped := strings.ReplaceAll(msg, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	return fmt.Sprintf(`{"time":"%s","level":"%s","msg":"%s"}`, t.Format("2006-01-02T15:04:05.000Z07:00"), level.String(), escaped)
}

// ============================================================
// 轮转检查
// ============================================================

func (m *Manager) rotationLoop() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(time.Duration(m.config.RotationCheckSec) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if m.fileWriter != nil {
				m.fileWriter.CheckRotation()
			}
		case <-m.stopCh:
			return
		}
	}
}

// ============================================================
// 日志轮转写入器
// ============================================================

// RotateConfig 轮转配置
type RotateConfig struct {
	MaxFileSize      int64 // 单文件最大字节数
	MaxFileCount     int   // 最大文件数
	MaxRetentionDays int   // 最大保留天数
	MaxTotalSize     int64 // 总大小限制
	EnableCompression bool  // 启用压缩
}

// RotateWriter 轮转写入器
type RotateWriter struct {
	config     *RotateConfig
	basePath   string
	currentFile *os.File
	currentSize int64
	mu         sync.Mutex
}

// NewRotateWriter 创建轮转写入器
func NewRotateWriter(path string, config *RotateConfig) (*RotateWriter, error) {
	if config == nil {
		config = &RotateConfig{
			MaxFileSize:      100 * 1024 * 1024,
			MaxFileCount:     10,
			MaxRetentionDays: 7,
			MaxTotalSize:     1024 * 1024 * 1024,
		}
	}
	
	rw := &RotateWriter{
		config:   config,
		basePath: path,
	}
	
	if err := rw.openNewFile(); err != nil {
		return nil, err
	}
	
	return rw, nil
}

// Write 写入数据
func (rw *RotateWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	
	// 检查是否需要轮转
	if rw.currentSize+int64(len(p)) > rw.config.MaxFileSize {
		rw.rotate()
	}
	
	n, err = rw.currentFile.Write(p)
	rw.currentSize += int64(n)
	return n, err
}

// CheckRotation 检查并执行轮转
func (rw *RotateWriter) CheckRotation() {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	
	if rw.currentSize >= rw.config.MaxFileSize {
		rw.rotate()
	}
	
	// 清理过期文件
	rw.cleanup()
}

// rotate 执行轮转
func (rw *RotateWriter) rotate() {
	// 关闭当前文件
	if rw.currentFile != nil {
		rw.currentFile.Close()
	}
	
	// 重命名当前文件
	if _, err := os.Stat(rw.basePath); err == nil {
		timestamp := time.Now().Format("20060102_150405")
		newName := rw.basePath + "." + timestamp
		os.Rename(rw.basePath, newName)
		
		// 压缩旧文件
		if rw.config.EnableCompression {
			go rw.compressFile(newName)
		}
	}
	
	// 打开新文件
	rw.openNewFile()
}

// openNewFile 打开新文件
func (rw *RotateWriter) openNewFile() error {
	file, err := os.OpenFile(rw.basePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	
	rw.currentFile = file
	rw.currentSize = 0
	return nil
}

// cleanup 清理过期文件
func (rw *RotateWriter) cleanup() {
	dir := filepath.Dir(rw.basePath)
	baseName := filepath.Base(rw.basePath)
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	
	now := time.Now()
	var files []string
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		if !strings.HasPrefix(name, baseName) {
			continue
		}
		
		// 检查是否过期
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		// 按天数清理
		if rw.config.MaxRetentionDays > 0 {
			if now.Sub(info.ModTime()) > time.Duration(rw.config.MaxRetentionDays)*24*time.Hour {
				os.Remove(filepath.Join(dir, name))
				continue
			}
		}
		
		files = append(files, filepath.Join(dir, name))
	}
	
	// 按文件数清理（保留最新的N个）
	if rw.config.MaxFileCount > 0 && len(files) > rw.config.MaxFileCount {
		// 按修改时间排序
		sort.Slice(files, func(i, j int) bool {
			fi, _ := os.Stat(files[i])
			fj, _ := os.Stat(files[j])
			return fi.ModTime().After(fj.ModTime())
		})
		
		// 删除最旧的文件
		for _, f := range files[rw.config.MaxFileCount:] {
			os.Remove(f)
		}
	}
	
	// 按总大小清理
	if rw.config.MaxTotalSize > 0 {
		var totalSize int64
		for _, f := range files {
			info, _ := os.Stat(f)
			if info != nil {
				totalSize += info.Size()
			}
		}
		
		if totalSize > rw.config.MaxTotalSize {
			// 按时间排序，删除最旧的
			sort.Slice(files, func(i, j int) bool {
				fi, _ := os.Stat(files[i])
				fj, _ := os.Stat(files[j])
				return fi.ModTime().After(fj.ModTime())
			})
			
			for _, f := range files {
				if totalSize <= rw.config.MaxTotalSize {
					break
				}
				info, _ := os.Stat(f)
				if info != nil {
					totalSize -= info.Size()
					os.Remove(f)
				}
			}
		}
	}
}

// compressFile 压缩文件
func (rw *RotateWriter) compressFile(path string) {
	// 简化实现：实际应使用gzip
	// 这里仅重命名标记
	newPath := path + ".gz"
	os.Rename(path, newPath)
}

// Close 关闭写入器
func (rw *RotateWriter) Close() error {
	if rw.currentFile != nil {
		return rw.currentFile.Close()
	}
	return nil
}

// ============================================================
// 全局日志管理器
// ============================================================

var globalManager *Manager
var globalMu sync.Mutex

// Init 初始化全局日志管理器
func Init(config *Config) error {
	globalMu.Lock()
	defer globalMu.Unlock()
	
	m, err := NewManager(config)
	if err != nil {
		return err
	}
	globalManager = m
	return nil
}

// GetManager 获取全局管理器
func GetManager() *Manager {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalManager
}

// 全局便捷函数
func Debug(format string, args ...interface{}) {
	if m := GetManager(); m != nil {
		m.Debug(format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if m := GetManager(); m != nil {
		m.Info(format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if m := GetManager(); m != nil {
		m.Warn(format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if m := GetManager(); m != nil {
		m.Error(format, args...)
	}
}

func Fatal(format string, args ...interface{}) {
	if m := GetManager(); m != nil {
		m.Fatal(format, args...)
	}
}
