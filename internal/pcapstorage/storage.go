// Package pcapstorage 全包存储引擎
// 支持指定时间窗口抓包、压缩存储、索引管理
package pcapstorage

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ============================================================
// 存储模型
// ============================================================

// PacketRecord 数据包记录
type PacketRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	NanoSec     uint32    `json:"nanosec"`      // 纳秒部分
	CapturedLen uint32    `json:"captured_len"` // 捕获长度
	OriginalLen uint32    `json:"original_len"` // 原始长度
	Data        []byte    `json:"data"`         // 数据包内容
	
	// 解析后的元数据（用于索引）
	SrcIP   string `json:"src_ip,omitempty"`
	DstIP   string `json:"dst_ip,omitempty"`
	SrcPort uint16 `json:"src_port,omitempty"`
	DstPort uint16 `json:"dst_port,omitempty"`
	Proto   uint8  `json:"proto,omitempty"` // 6=TCP, 17=UDP
}

// Block 存储块（按时间分块）
type Block struct {
	ID          string    `json:"id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	FilePath    string    `json:"file_path"`
	PacketCount int64     `json:"packet_count"`
	ByteSize    int64     `json:"byte_size"`
	Compressed  bool      `json:"compressed"`
	IndexPath   string    `json:"index_path"`
}

// Query 查询条件
type Query struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	SrcIP     string    `json:"src_ip,omitempty"`
	DstIP     string    `json:"dst_ip,omitempty"`
	Port      uint16    `json:"port,omitempty"`
	Proto     uint8     `json:"proto,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

// ============================================================
// 存储引擎
// ============================================================

// Engine 抓包存储引擎
type Engine struct {
	config      *StorageConfig
	baseDir     string
	currentBlock *Block
	blockWriter io.WriteCloser
	indexWriter *IndexWriter
	
	blocks      map[string]*Block // 所有块索引
	blocksMu    sync.RWMutex
	
	writeQueue  chan *PacketRecord
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// StorageConfig 存储配置
type StorageConfig struct {
	BaseDir           string        `yaml:"base_dir" json:"base_dir"`                     // 存储根目录
	BlockDuration     time.Duration `yaml:"block_duration" json:"block_duration"`         // 每个块的时间跨度
	MaxBlockSize      int64         `yaml:"max_block_size" json:"max_block_size"`         // 单个块最大大小 (MB)
	CompressEnabled   bool          `yaml:"compress_enabled" json:"compress_enabled"`     // 启用压缩
	CompressLevel     int           `yaml:"compress_level" json:"compress_level"`         // 压缩级别 1-9
	IndexEnabled      bool          `yaml:"index_enabled" json:"index_enabled"`           // 启用索引
	MaxRetention      time.Duration `yaml:"max_retention" json:"max_retention"`           // 最大保留时间
	MaxTotalSize      int64         `yaml:"max_total_size" json:"max_total_size"`         // 最大总容量 (GB)
	WriteBufferSize   int           `yaml:"write_buffer_size" json:"write_buffer_size"`   // 写缓冲区大小
	FlushInterval     time.Duration `yaml:"flush_interval" json:"flush_interval"`         // 刷新间隔
}

// DefaultConfig 默认配置
func DefaultConfig() *StorageConfig {
	return &StorageConfig{
		BaseDir:         "/var/lib/cloud-flow/pcap",
		BlockDuration:   5 * time.Minute,
		MaxBlockSize:    100, // 100MB
		CompressEnabled: true,
		CompressLevel:   6,
		IndexEnabled:    true,
		MaxRetention:    7 * 24 * time.Hour, // 7天
		MaxTotalSize:    100, // 100GB
		WriteBufferSize: 1000,
		FlushInterval:   5 * time.Second,
	}
}

// NewEngine 创建存储引擎
func NewEngine(config *StorageConfig) (*Engine, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	// 创建存储目录
	if err := os.MkdirAll(config.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	
	engine := &Engine{
		config:     config,
		baseDir:    config.BaseDir,
		blocks:     make(map[string]*Block),
		writeQueue: make(chan *PacketRecord, config.WriteBufferSize),
		stopCh:     make(chan struct{}),
	}
	
	// 加载已有块
	if err := engine.loadExistingBlocks(); err != nil {
		return nil, err
	}
	
	// 启动写入协程
	engine.wg.Add(1)
	go engine.writeLoop()
	
	// 启动清理协程
	engine.wg.Add(1)
	go engine.cleanupLoop()
	
	return engine, nil
}

// ============================================================
// 数据包写入
// ============================================================

// WritePacket 写入数据包（异步）
func (e *Engine) WritePacket(record *PacketRecord) error {
	select {
	case e.writeQueue <- record:
		return nil
	case <-time.After(time.Second):
		return fmt.Errorf("写入队列已满")
	}
}

// WritePacketSync 同步写入数据包
func (e *Engine) WritePacketSync(record *PacketRecord) error {
	e.blocksMu.Lock()
	defer e.blocksMu.Unlock()
	
	// 检查是否需要创建新块
	if e.currentBlock == nil || e.needNewBlock(record.Timestamp) {
		if err := e.rotateBlock(); err != nil {
			return err
		}
	}
	
	// 写入数据
	if err := e.writeToBlock(record); err != nil {
		return err
	}
	
	// 更新块元数据
	e.currentBlock.PacketCount++
	e.currentBlock.EndTime = record.Timestamp
	
	return nil
}

// writeLoop 异步写入循环
func (e *Engine) writeLoop() {
	defer e.wg.Done()
	
	ticker := time.NewTicker(e.config.FlushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case record := <-e.writeQueue:
			if err := e.WritePacketSync(record); err != nil {
				// 记录错误日志
			}
			
		case <-ticker.C:
			e.flush()
			
		case <-e.stopCh:
			e.flush()
			return
		}
	}
}

// needNewBlock 检查是否需要创建新块
func (e *Engine) needNewBlock(ts time.Time) bool {
	if e.currentBlock == nil {
		return true
	}
	
	// 时间跨度超过限制
	if ts.Sub(e.currentBlock.StartTime) > e.config.BlockDuration {
		return true
	}
	
	// 块大小超过限制
	if e.currentBlock.ByteSize > e.config.MaxBlockSize*1024*1024 {
		return true
	}
	
	return false
}

// rotateBlock 轮转创建新块
func (e *Engine) rotateBlock() error {
	// 关闭当前块
	if e.blockWriter != nil {
		e.blockWriter.Close()
		e.blockWriter = nil
	}
	if e.indexWriter != nil {
		e.indexWriter.Close()
		e.indexWriter = nil
	}
	
	// 保存当前块信息
	if e.currentBlock != nil {
		e.blocks[e.currentBlock.ID] = e.currentBlock
		if err := e.saveBlockMeta(e.currentBlock); err != nil {
			return err
		}
	}
	
	// 创建新块
	now := time.Now()
	blockID := now.Format("20060102_150405")
	
	block := &Block{
		ID:         blockID,
		StartTime:  now,
		FilePath:   filepath.Join(e.baseDir, fmt.Sprintf("%s.pcap", blockID)),
		Compressed: e.config.CompressEnabled,
		IndexPath:  filepath.Join(e.baseDir, fmt.Sprintf("%s.idx", blockID)),
	}
	
	// 创建文件
	file, err := os.Create(block.FilePath)
	if err != nil {
		return fmt.Errorf("创建块文件失败: %w", err)
	}
	
	// 写入PCAP文件头
	if err := writePcapHeader(file); err != nil {
		file.Close()
		return err
	}
	
	// 包装压缩写入器
	if e.config.CompressEnabled {
		e.blockWriter = &gzipWriter{
			gw:   gzip.NewWriter(file),
			file: file,
		}
	} else {
		e.blockWriter = file
	}
	
	// 创建索引写入器
	if e.config.IndexEnabled {
		e.indexWriter = NewIndexWriter(block.IndexPath)
	}
	
	e.currentBlock = block
	return nil
}

// writeToBlock 写入数据到当前块
func (e *Engine) writeToBlock(record *PacketRecord) error {
	// 写入PCAP记录头
	header := make([]byte, 16)
	binary.LittleEndian.PutUint32(header[0:4], uint32(record.Timestamp.Unix()))
	binary.LittleEndian.PutUint32(header[4:8], record.NanoSec)
	binary.LittleEndian.PutUint32(header[8:12], record.CapturedLen)
	binary.LittleEndian.PutUint32(header[12:16], record.OriginalLen)
	
	if _, err := e.blockWriter.Write(header); err != nil {
		return err
	}
	
	// 写入数据
	if _, err := e.blockWriter.Write(record.Data); err != nil {
		return err
	}
	
	e.currentBlock.ByteSize += int64(len(header) + len(record.Data))
	
	// 写入索引
	if e.indexWriter != nil {
		e.indexWriter.Write(record)
	}
	
	return nil
}

// flush 刷新缓冲区
func (e *Engine) flush() {
	e.blocksMu.Lock()
	defer e.blocksMu.Unlock()
	
	if e.blockWriter != nil {
		if flusher, ok := e.blockWriter.(interface{ Flush() error }); ok {
			flusher.Flush()
		}
	}
}

// ============================================================
// 查询与读取
// ============================================================

// Query 按条件查询数据包
func (e *Engine) Query(q *Query) ([]*PacketRecord, error) {
	e.blocksMu.RLock()
	defer e.blocksMu.RUnlock()
	
	var results []*PacketRecord
	
	// 确定需要查询的块
	blocks := e.selectBlocks(q.StartTime, q.EndTime)
	
	for _, block := range blocks {
		records, err := e.queryBlock(block, q)
		if err != nil {
			continue // 跳过错误块
		}
		results = append(results, records...)
		
		if q.Limit > 0 && len(results) >= q.Limit {
			results = results[:q.Limit]
			break
		}
	}
	
	return results, nil
}

// selectBlocks 选择时间范围内的块
func (e *Engine) selectBlocks(start, end time.Time) []*Block {
	var selected []*Block
	
	for _, block := range e.blocks {
		// 块的时间范围与查询范围有交集
		if block.EndTime.After(start) && block.StartTime.Before(end) {
			selected = append(selected, block)
		}
	}
	
	// 包含当前正在写入的块
	if e.currentBlock != nil {
		if e.currentBlock.EndTime.After(start) && e.currentBlock.StartTime.Before(end) {
			selected = append(selected, e.currentBlock)
		}
	}
	
	return selected
}

// queryBlock 查询单个块
func (e *Engine) queryBlock(block *Block, q *Query) ([]*PacketRecord, error) {
	// 打开文件
	file, err := os.Open(block.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var reader io.Reader = file
	
	// 如果是压缩文件，解压读取
	if block.Compressed {
		gr, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		reader = gr
	}
	
	// 跳过PCAP头
	header := make([]byte, 24)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	
	var records []*PacketRecord
	
	// 读取所有记录
	for {
		record, err := readPcapRecord(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		
		// 时间过滤
		if record.Timestamp.Before(q.StartTime) || record.Timestamp.After(q.EndTime) {
			continue
		}
		
		// IP过滤
		if q.SrcIP != "" && record.SrcIP != q.SrcIP {
			continue
		}
		if q.DstIP != "" && record.DstIP != q.DstIP {
			continue
		}
		
		// 端口过滤
		if q.Port != 0 && record.SrcPort != q.Port && record.DstPort != q.Port {
			continue
		}
		
		// 协议过滤
		if q.Proto != 0 && record.Proto != q.Proto {
			continue
		}
		
		records = append(records, record)
	}
	
	return records, nil
}

// GetBlockInfo 获取块信息列表
func (e *Engine) GetBlockInfo() []*Block {
	e.blocksMu.RLock()
	defer e.blocksMu.RUnlock()
	
	blocks := make([]*Block, 0, len(e.blocks))
	for _, b := range e.blocks {
		blocks = append(blocks, b)
	}
	
	// 包含当前块
	if e.currentBlock != nil {
		blocks = append(blocks, e.currentBlock)
	}
	
	return blocks
}

// ============================================================
// 清理与维护
// ============================================================

// cleanupLoop 清理循环
func (e *Engine) cleanupLoop() {
	defer e.wg.Done()
	
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			e.cleanup()
		case <-e.stopCh:
			e.cleanup()
			return
		}
	}
}

// cleanup 执行清理
func (e *Engine) cleanup() {
	e.blocksMu.Lock()
	defer e.blocksMu.Unlock()
	
	now := time.Now()
	
	// 按保留时间清理
	for id, block := range e.blocks {
		if now.Sub(block.EndTime) > e.config.MaxRetention {
			os.Remove(block.FilePath)
			os.Remove(block.IndexPath)
			delete(e.blocks, id)
		}
	}
	
	// 按总容量清理（删除最旧的块）
	var totalSize int64
	for _, block := range e.blocks {
		totalSize += block.ByteSize
	}
	
	maxSize := e.config.MaxTotalSize * 1024 * 1024 * 1024
	if totalSize > maxSize {
		// 按时间排序，删除最旧的
		var sorted []*Block
		for _, b := range e.blocks {
			sorted = append(sorted, b)
		}
		
		// 简单冒泡排序
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i].StartTime.After(sorted[j].StartTime) {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		
		for _, block := range sorted {
			if totalSize <= maxSize {
				break
			}
			os.Remove(block.FilePath)
			os.Remove(block.IndexPath)
			delete(e.blocks, block.ID)
			totalSize -= block.ByteSize
		}
	}
}

// ============================================================
// 辅助函数
// ============================================================

// writePcapHeader 写入PCAP文件头
func writePcapHeader(w io.Writer) error {
	header := make([]byte, 24)
	// Magic Number (little endian)
	binary.LittleEndian.PutUint32(header[0:4], 0xa1b2c3d4)
	// Version Major
	binary.LittleEndian.PutUint16(header[4:6], 2)
	// Version Minor
	binary.LittleEndian.PutUint16(header[6:8], 4)
	// Timezone (GMT)
	binary.LittleEndian.PutUint32(header[8:12], 0)
	// Sigfigs
	binary.LittleEndian.PutUint32(header[12:16], 0)
	// Snaplen
	binary.LittleEndian.PutUint32(header[16:20], 65535)
	// Network (Ethernet)
	binary.LittleEndian.PutUint32(header[20:24], 1)
	
	_, err := w.Write(header)
	return err
}

// readPcapRecord 读取PCAP记录
func readPcapRecord(r io.Reader) (*PacketRecord, error) {
	header := make([]byte, 16)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	
	record := &PacketRecord{
		Timestamp:   time.Unix(int64(binary.LittleEndian.Uint32(header[0:4])), 0),
		NanoSec:     binary.LittleEndian.Uint32(header[4:8]),
		CapturedLen: binary.LittleEndian.Uint32(header[8:12]),
		OriginalLen: binary.LittleEndian.Uint32(header[12:16]),
	}
	
	record.Data = make([]byte, record.CapturedLen)
	if _, err := io.ReadFull(r, record.Data); err != nil {
		return nil, err
	}
	
	return record, nil
}

// loadExistingBlocks 加载已有块
func (e *Engine) loadExistingBlocks() error {
	entries, err := os.ReadDir(e.baseDir)
	if err != nil {
		return err
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		if filepath.Ext(name) != ".pcap" {
			continue
		}
		
		blockID := name[:len(name)-5]
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		// 解析块ID获取时间
		t, err := time.Parse("20060102_150405", blockID)
		if err != nil {
			continue
		}
		
		block := &Block{
			ID:         blockID,
			StartTime:  t,
			EndTime:    t.Add(e.config.BlockDuration),
			FilePath:   filepath.Join(e.baseDir, name),
			ByteSize:   info.Size(),
			Compressed: false, // 从文件扩展名判断
			IndexPath:  filepath.Join(e.baseDir, blockID+".idx"),
		}
		
		e.blocks[blockID] = block
	}
	
	return nil
}

// saveBlockMeta 保存块元数据
func (e *Engine) saveBlockMeta(block *Block) error {
	// 简化实现：元数据直接存储在文件名中
	// 实际生产环境可使用单独的元数据文件或数据库
	return nil
}

// Close 关闭引擎
func (e *Engine) Close() error {
	close(e.stopCh)
	e.wg.Wait()
	
	e.blocksMu.Lock()
	defer e.blocksMu.Unlock()
	
	if e.blockWriter != nil {
		e.blockWriter.Close()
	}
	if e.indexWriter != nil {
		e.indexWriter.Close()
	}
	
	return nil
}

// ============================================================
// gzipWriter 包装器
// ============================================================

type gzipWriter struct {
	gw   *gzip.Writer
	file *os.File
}

func (w *gzipWriter) Write(p []byte) (n int, err error) {
	return w.gw.Write(p)
}

func (w *gzipWriter) Close() error {
	w.gw.Close()
	return w.file.Close()
}

func (w *gzipWriter) Flush() error {
	return w.gw.Flush()
}
