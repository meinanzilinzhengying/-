/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package storage

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pierrec/lz4/v4"

	"cloud-flow-agent/pkg/logger"
)

// CompressionType 压缩算法类型
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGZip CompressionType = "gzip"
	CompressionZSTD CompressionType = "zstd"
	CompressionLZ4  CompressionType = "lz4"
)

// CompressionLevel 压缩级别
type CompressionLevel int

const (
	CompressionLevelFast    CompressionLevel = 1
	CompressionLevelDefault CompressionLevel = 6
	CompressionLevelBest   CompressionLevel = 9
)

// CompressionConfig 压缩配置
type CompressionConfig struct {
	Enabled           bool            `yaml:"enabled" json:"enabled"`
	Algorithm         CompressionType `yaml:"algorithm" json:"algorithm"`             // gzip/zstd/lz4
	Level             CompressionLevel `yaml:"level" json:"level"`                     // 压缩级别 1-9
	BlockSize         int             `yaml:"block_size" json:"block_size"`           // 块大小 (bytes)
	MinSizeToCompress int64           `yaml:"min_size_to_compress" json:"min_size_to_compress"` // 最小压缩大小
	ThresholdRatio    float64         `yaml:"threshold_ratio" json:"threshold_ratio"` // 压缩率阈值，低于此值不压缩
}

// DefaultCompressionConfig 默认压缩配置
func DefaultCompressionConfig() *CompressionConfig {
	return &CompressionConfig{
		Enabled:           true,
		Algorithm:         CompressionGZip, // 使用 gzip 保证兼容性
		Level:             CompressionLevelDefault,
		BlockSize:         256 * 1024, // 256KB
		MinSizeToCompress: 1024,       // 小于1KB不压缩
		ThresholdRatio:    0.8,        // 压缩后大小超过80%则不保留
	}
}

// Compressor 压缩器接口
type Compressor interface {
	Compress(src []byte) ([]byte, error)
	Decompress(src []byte) ([]byte, error)
	CompressStream(src io.Reader, dst io.Writer) error
	DecompressStream(src io.Reader, dst io.Writer) error
	Name() string
}

// GZipCompressor GZip压缩器
type GZipCompressor struct {
	level CompressionLevel
}

// NewGZipCompressor 创建GZip压缩器
func NewGZipCompressor(level CompressionLevel) *GZipCompressor {
	return &GZipCompressor{level: level}
}

// Name 返回压缩器名称
func (c *GZipCompressor) Name() string {
	return "gzip"
}

// Compress 压缩数据
func (c *GZipCompressor) Compress(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, nil
	}

	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, int(c.level))
	if err != nil {
		return nil, fmt.Errorf("create gzip writer: %w", err)
	}

	if _, err := writer.Write(src); err != nil {
		return nil, fmt.Errorf("write gzip: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %w", err)
	}

	return buf.Bytes(), nil
}

// Decompress 解压数据
func (c *GZipCompressor) Decompress(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, nil
	}

	reader, err := gzip.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("create gzip reader: %w", err)
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// CompressStream 流式压缩
func (c *GZipCompressor) CompressStream(src io.Reader, dst io.Writer) error {
	writer, err := gzip.NewWriterLevel(dst, int(c.level))
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}

	if _, err := io.Copy(writer, src); err != nil {
		writer.Close()
		return fmt.Errorf("copy data: %w", err)
	}

	return writer.Close()
}

// DecompressStream 流式解压
func (c *GZipCompressor) DecompressStream(src io.Reader, dst io.Writer) error {
	reader, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer reader.Close()

	if _, err := io.Copy(dst, reader); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return nil
}

// LZ4Compressor LZ4压缩器 (高速压缩)
type LZ4Compressor struct {
	level lz4.CompressionLevel
}

// NewLZ4Compressor 创建LZ4压缩器
func NewLZ4Compressor(level CompressionLevel) *LZ4Compressor {
	// 映射 1-9 到 LZ4 级别 (0-9)
	lv := lz4.CompressionLevel(256 - int(level)*25)
	if lv < lz4.Fast {
		lv = lz4.Fast
	}
	if lv > lz4.HighCompression {
		lv = lz4.HighCompression
	}
	return &LZ4Compressor{level: lv}
}

// Name 返回压缩器名称
func (c *LZ4Compressor) Name() string {
	return "lz4"
}

// Compress 压缩数据
func (c *LZ4Compressor) Compress(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, nil
	}

	dst := make([]byte, len(src)*2) // LZ4 压缩后通常比源数据小
	n, err := lz4.CompressBlock(src, dst, 0)
	if err != nil {
		return nil, fmt.Errorf("lz4 compress: %w", err)
	}

	return dst[:n], nil
}

// Decompress 解压数据
func (c *LZ4Compressor) Decompress(src []byte) ([]byte, error) {
	if len(src) == 0 {
		return nil, nil
	}

	// LZ4 block 解压需要原始大小信息，这里简化处理返回错误
	return nil, fmt.Errorf("lz4 block decompression requires size information, use stream mode")
}

// CompressStream 流式压缩
func (c *LZ4Compressor) CompressStream(src io.Reader, dst io.Writer) error {
	writer := lz4.NewWriter(dst)
	writer.CompressionLevel()

	buf := make([]byte, 64*1024) // 64KB buffer
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, err := writer.Write(buf[:n]); err != nil {
				return fmt.Errorf("write: %w", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
	}

	return writer.Close()
}

// DecompressStream 流式解压
func (c *LZ4Compressor) DecompressStream(src io.Reader, dst io.Writer) error {
	reader := lz4.NewReader(src)
	buf := make([]byte, 64*1024)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return fmt.Errorf("write: %w", err)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
	}

	return nil
}

// CompressorFactory 压缩器工厂
type CompressorFactory struct {
	config *CompressionConfig
	mu     sync.RWMutex
}

// NewCompressorFactory 创建压缩器工厂
func NewCompressorFactory(config *CompressionConfig) *CompressorFactory {
	if config == nil {
		config = DefaultCompressionConfig()
	}
	return &CompressorFactory{config: config}
}

// GetCompressor 获取压缩器
func (f *CompressorFactory) GetCompressor() Compressor {
	f.mu.RLock()
	defer f.mu.RUnlock()

	switch f.config.Algorithm {
	case CompressionGZip:
		return NewGZipCompressor(f.config.Level)
	case CompressionLZ4:
		return NewLZ4Compressor(f.config.Level)
	default:
		return &NoOpCompressor{}
	}
}

// UpdateConfig 更新配置
func (f *CompressorFactory) UpdateConfig(config *CompressionConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config = config
}

// NoOpCompressor 无压缩器
type NoOpCompressor struct{}

// Name 返回压缩器名称
func (c *NoOpCompressor) Name() string {
	return "none"
}

// Compress 直接返回原数据
func (c *NoOpCompressor) Compress(src []byte) ([]byte, error) {
	return src, nil
}

// Decompress 直接返回原数据
func (c *NoOpCompressor) Decompress(src []byte) ([]byte, error) {
	return src, nil
}

// CompressStream 流式复制
func (c *NoOpCompressor) CompressStream(src io.Reader, dst io.Writer) error {
	_, err := io.Copy(dst, src)
	return err
}

// DecompressStream 流式复制
func (c *NoOpCompressor) DecompressStream(src io.Reader, dst io.Writer) error {
	_, err := io.Copy(dst, src)
	return err
}

// CompressedFile 压缩文件包装器
type CompressedFile struct {
	path       string
	compressor Compressor
	config     *CompressionConfig
}

// NewCompressedFile 创建压缩文件
func NewCompressedFile(path string, config *CompressionConfig) *CompressedFile {
	if config == nil {
		config = DefaultCompressionConfig()
	}

	factory := NewCompressorFactory(config)
	return &CompressedFile{
		path:       path,
		compressor: factory.GetCompressor(),
		config:     config,
	}
}

// Write 压缩并写入文件
func (f *CompressedFile) Write(data []byte) error {
	if !f.config.Enabled {
		return os.WriteFile(f.path, data, 0644)
	}

	// 小于最小压缩大小直接写入
	if int64(len(data)) < f.config.MinSizeToCompress {
		return os.WriteFile(f.path, data, 0644)
	}

	compressed, err := f.compressor.Compress(data)
	if err != nil {
		return fmt.Errorf("compress data: %w", err)
	}

	// 检查压缩率
	ratio := float64(len(compressed)) / float64(len(data))
	if ratio > f.config.ThresholdRatio {
		logger.Debugf("Compression ratio %.2f exceeds threshold %.2f, storing uncompressed",
			ratio, f.config.ThresholdRatio)
		return os.WriteFile(f.path, data, 0644)
	}

	return os.WriteFile(f.path, compressed, 0644)
}

// Read 读取并解压文件
func (f *CompressedFile) Read() ([]byte, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	if !f.config.Enabled {
		return data, nil
	}

	// 尝试解压，如果失败则原样返回
	decompressed, err := f.compressor.Decompress(data)
	if err != nil {
		logger.Debugf("Decompression failed, returning raw data: %v", err)
		return data, nil
	}

	return decompressed, nil
}

// WriteStream 流式压缩写入
func (f *CompressedFile) WriteStream(reader io.Reader) error {
	if !f.config.Enabled {
		file, err := os.Create(f.path)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
		defer file.Close()
		_, err = io.Copy(file, reader)
		return err
	}

	file, err := os.Create(f.path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	return f.compressor.CompressStream(reader, file)
}

// ReadStream 流式解压读取
func (f *CompressedFile) ReadStream(writer io.Writer) error {
	file, err := os.Open(f.path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	if !f.config.Enabled {
		_, err := io.Copy(writer, file)
		return err
	}

	return f.compressor.DecompressStream(file, writer)
}

// CompressFile 直接压缩文件
func CompressFile(srcPath, dstPath string, config *CompressionConfig) error {
	if config == nil {
		config = DefaultCompressionConfig()
	}

	factory := NewCompressorFactory(config)
	compressor := factory.GetCompressor()

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	// 检查文件大小
	info, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	// 小文件不压缩
	if info.Size() < config.MinSizeToCompress {
		if dstPath == "" {
			dstPath = srcPath
		}
		_, err := io.Copy(os.Create(dstPath), srcFile)
		return err
	}

	if dstPath == "" {
		dstPath = srcPath + ".gz"
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dstFile.Close()

	return compressor.CompressStream(srcFile, dstFile)
}

// DecompressFile 直接解压文件
func DecompressFile(srcPath, dstPath string, config *CompressionConfig) error {
	if config == nil {
		config = DefaultCompressionConfig()
	}

	factory := NewCompressorFactory(config)
	compressor := factory.GetCompressor()

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	if dstPath == "" {
		dstPath = srcPath
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dstFile.Close()

	return compressor.DecompressStream(srcFile, dstFile)
}

// CompressionStats 压缩统计
type CompressionStats struct {
	TotalOriginalSize  uint64  `json:"total_original_size"`
	TotalCompressedSize uint64 `json:"total_compressed_size"`
	CompressionRatio   float64 `json:"compression_ratio"`
	FilesCompressed    uint64  `json:"files_compressed"`
	FilesSkipped       uint64  `json:"files_skipped"`
}

// DirectoryCompressor 目录压缩器
type DirectoryCompressor struct {
	factory *CompressorFactory
	config  *CompressionConfig
	stats   CompressionStats
	statsMu sync.Mutex
}

// NewDirectoryCompressor 创建目录压缩器
func NewDirectoryCompressor(config *CompressionConfig) *DirectoryCompressor {
	if config == nil {
		config = DefaultCompressionConfig()
	}
	return &DirectoryCompressor{
		factory: NewCompressorFactory(config),
		config:  config,
	}
}

// CompressDirectory 压缩整个目录
func (dc *DirectoryCompressor) CompressDirectory(srcDir, dstDir string, extensions []string) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create dst dir: %w", err)
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		if info.IsDir() {
			return nil
		}

		// 检查扩展名
		if len(extensions) > 0 {
			ext := filepath.Ext(path)
			found := false
			for _, e := range extensions {
				if ext == e {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return nil
		}

		dstPath := filepath.Join(dstDir, relPath) + ".gz"
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		return dc.compressFile(path, dstPath)
	})
}

// compressFile 压缩单个文件
func (dc *DirectoryCompressor) compressFile(srcPath, dstPath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	originalSize := info.Size()

	// 小文件跳过
	if originalSize < dc.config.MinSizeToCompress {
		dc.statsMu.Lock()
		dc.stats.FilesSkipped++
		dc.stats.TotalOriginalSize += uint64(originalSize)
		dc.statsMu.Unlock()
		return nil
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	compressor := dc.factory.GetCompressor()
	if err := compressor.CompressStream(srcFile, dstFile); err != nil {
		return err
	}

	// 获取压缩后大小
	compressedInfo, err := dstFile.Stat()
	if err != nil {
		return err
	}
	compressedSize := compressedInfo.Size()

	dc.statsMu.Lock()
	dc.stats.FilesCompressed++
	dc.stats.TotalOriginalSize += uint64(originalSize)
	dc.stats.TotalCompressedSize += uint64(compressedSize)
	if dc.stats.TotalOriginalSize > 0 {
		dc.stats.CompressionRatio = float64(dc.stats.TotalCompressedSize) / float64(dc.stats.TotalOriginalSize)
	}
	dc.statsMu.Unlock()

	logger.Infof("Compressed %s: %d -> %d bytes (ratio: %.2f%%)",
		srcPath, originalSize, compressedSize, float64(compressedSize)/float64(originalSize)*100)

	return nil
}

// GetStats 获取压缩统计
func (dc *DirectoryCompressor) GetStats() CompressionStats {
	dc.statsMu.Lock()
	defer dc.statsMu.Unlock()
	return dc.stats
}

// Header 压缩文件头
type Header struct {
	Magic       [4]byte // 魔数 "CFS\0"
	Version     uint8   // 版本号
	Algorithm   uint8   // 算法
	Level       uint8   // 压缩级别
	Flags       uint8   // 标志位
	OrigSize    uint64  // 原始大小
	CompSize    uint64  // 压缩后大小
	ModTime     uint64  // 修改时间
	OrigNameLen uint16  // 原始文件名长度
}

// MagicBytes 压缩文件魔数
var MagicBytes = [4]byte{'C', 'F', 'S', 0}

// WriteCompressedFile 写入带头的压缩文件
func WriteCompressedFile(path string, data []byte, config *CompressionConfig) error {
	if config == nil {
		config = DefaultCompressionConfig()
	}

	factory := NewCompressorFactory(config)
	compressor := factory.GetCompressor()

	// 压缩数据
	compressed, err := compressor.Compress(data)
	if err != nil {
		return fmt.Errorf("compress: %w", err)
	}

	// 构建文件头
	origName := filepath.Base(path)
	header := Header{
		Magic:       MagicBytes,
		Version:     1,
		Algorithm:   uint8(config.Algorithm),
		Level:       uint8(config.Level),
		Flags:       0,
		OrigSize:    uint64(len(data)),
		CompSize:    uint64(len(compressed)),
		OrigNameLen: uint16(len(origName)),
	}

	// 写入文件
	file, err := os.Create(path + ".cf")
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if err := binary.Write(file, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	if _, err := file.WriteString(origName); err != nil {
		return fmt.Errorf("write name: %w", err)
	}

	if _, err := file.Write(compressed); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

// ReadCompressedFile 读取带头压缩的文件
func ReadCompressedFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	var header Header
	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// 检查魔数
	if header.Magic != MagicBytes {
		// 不是压缩文件，直接读取
		return os.ReadFile(path)
	}

	// 读取原始文件名
	nameBytes := make([]byte, header.OrigNameLen)
	if _, err := file.Read(nameBytes); err != nil {
		return nil, fmt.Errorf("read name: %w", err)
	}

	// 读取压缩数据
	compressed := make([]byte, header.CompSize)
	if _, err := file.Read(compressed); err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}

	// 解压
	config := &CompressionConfig{Algorithm: CompressionType(header.Algorithm)}
	factory := NewCompressorFactory(config)
	compressor := factory.GetCompressor()

	return compressor.Decompress(compressed)
}

// DataWriter 数据写入器（带压缩）
type DataWriter struct {
	factory    *CompressorFactory
	config     *CompressionConfig
	currentDir string
	fileIndex  int64
	mu         sync.Mutex
}

// NewDataWriter 创建数据写入器
func NewDataWriter(baseDir string, config *CompressionConfig) (*DataWriter, error) {
	if config == nil {
		config = DefaultCompressionConfig()
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}

	return &DataWriter{
		factory:    NewCompressorFactory(config),
		config:     config,
		currentDir: baseDir,
		fileIndex:  0,
	}, nil
}

// Write 写入压缩数据
func (dw *DataWriter) Write(category string, data []byte) (string, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	// 创建分类目录
	dir := filepath.Join(dw.currentDir, category)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	// 生成文件名
	filename := fmt.Sprintf("data_%d.bin", dw.fileIndex)
	dw.fileIndex++

	ext := ".gz"
	if dw.config.Algorithm == CompressionLZ4 {
		ext = ".lz4"
	}
	filePath := filepath.Join(dir, filename+ext)

	// 压缩并写入
	if err := dw.writeCompressed(filePath, data); err != nil {
		return "", err
	}

	return filePath, nil
}

// writeCompressed 写入压缩数据
func (dw *DataWriter) writeCompressed(path string, data []byte) error {
	if !dw.config.Enabled {
		return os.WriteFile(path, data, 0644)
	}

	// 小于最小压缩大小不压缩
	if int64(len(data)) < dw.config.MinSizeToCompress {
		// 去掉扩展名写入
		noExt := path
		if ext := filepath.Ext(path); ext == ".gz" || ext == ".lz4" {
			noExt = path[:len(path)-len(ext)]
		}
		return os.WriteFile(noExt, data, 0644)
	}

	compressor := dw.factory.GetCompressor()
	compressed, err := compressor.Compress(data)
	if err != nil {
		// 压缩失败则写入原数据
		noExt := path
		if ext := filepath.Ext(path); ext == ".gz" || ext == ".lz4" {
			noExt = path[:len(path)-len(ext)]
		}
		return os.WriteFile(noExt, data, 0644)
	}

	// 检查压缩率
	ratio := float64(len(compressed)) / float64(len(data))
	if ratio > dw.config.ThresholdRatio {
		// 压缩率太低，写入原数据
		noExt := path
		if ext := filepath.Ext(path); ext == ".gz" || ext == ".lz4" {
			noExt = path[:len(path)-len(ext)]
		}
		return os.WriteFile(noExt, data, 0644)
	}

	return os.WriteFile(path, compressed, 0644)
}

// WriteStream 流式写入
func (dw *DataWriter) WriteStream(category string, reader io.Reader) (string, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	// 创建分类目录
	dir := filepath.Join(dw.currentDir, category)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	// 生成文件名
	filename := fmt.Sprintf("data_%d.bin", dw.fileIndex)
	dw.fileIndex++

	ext := ".gz"
	if dw.config.Algorithm == CompressionLZ4 {
		ext = ".lz4"
	}
	filePath := filepath.Join(dir, filename+ext)

	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if !dw.config.Enabled {
		_, err = io.Copy(file, reader)
		return filePath, err
	}

	compressor := dw.factory.GetCompressor()
	return filePath, compressor.CompressStream(reader, file)
}
