// Package jvmmem 提供Java内存分析功能
// 支持ByteBuffer和JNI内存统计、泄漏检测、调用栈定位
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package jvmmem

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
)

// ============================================================
// 配置定义
// ============================================================

// JVMMemConfig Java内存分析配置
type JVMMemConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	SampleRate        int           `yaml:"sample_rate" json:"sample_rate"`           // 采样率 (0-100%)
	TargetPIDs        []uint32      `yaml:"target_pids" json:"target_pids"`           // 目标JVM进程
	TrackByteBuffer   bool          `yaml:"track_bytebuffer" json:"track_bytebuffer"` // 追踪ByteBuffer
	TrackJNIMemory    bool          `yaml:"track_jni_memory" json:"track_jni_memory"` // 追踪JNI内存
	TrackDirectMemory bool          `yaml:"track_direct_memory" json:"track_direct_memory"` // 追踪堆外内存
	LeakCheckInterval time.Duration `yaml:"leak_check_interval" json:"leak_check_interval"` // 泄漏检查间隔
	LeakThreshold     float64       `yaml:"leak_threshold" json:"leak_threshold"`     // 泄漏阈值（增长率/小时）
	MinLeakSize       uint64        `yaml:"min_leak_size" json:"min_leak_size"`       // 最小泄漏大小（字节）
	MaxStackDepth     int           `yaml:"max_stack_depth" json:"max_stack_depth"`   // 最大栈深度
	SymbolResolution  bool          `yaml:"symbol_resolution" json:"symbol_resolution"` // 符号解析
	ReportInterval    time.Duration `yaml:"report_interval" json:"report_interval"`   // 报告间隔
}

func DefaultJVMMemConfig() *JVMMemConfig {
	return &JVMMemConfig{
		Enabled:           true,
		SampleRate:        100, // 100%采样
		TargetPIDs:        []uint32{},
		TrackByteBuffer:   true,
		TrackJNIMemory:    true,
		TrackDirectMemory: true,
		LeakCheckInterval: 5 * time.Minute,
		LeakThreshold:     10.0, // 10%/小时增长率视为泄漏
		MinLeakSize:       1024 * 1024, // 1MB
		MaxStackDepth:     64,
		SymbolResolution:  true,
		ReportInterval:    60 * time.Second,
	}
}

// ============================================================
// 数据模型
// ============================================================

// MemoryType 内存类型
type MemoryType int

const (
	MemTypeUnknown MemoryType = iota
	MemTypeByteBufferAllocate
	MemTypeByteBufferFree
	MemTypeJNIMalloc
	MemTypeJNIFree
	MemTypeDirectMemoryAllocate
	MemTypeDirectMemoryFree
	MemTypeUnsafeAllocate
	MemTypeUnsafeFree
)

func (m MemoryType) String() string {
	names := map[MemoryType]string{
		MemTypeByteBufferAllocate:   "ByteBuffer.allocate",
		MemTypeByteBufferFree:       "ByteBuffer.free",
		MemTypeJNIMalloc:            "JNI.malloc",
		MemTypeJNIFree:              "JNI.free",
		MemTypeDirectMemoryAllocate: "DirectMemory.allocate",
		MemTypeDirectMemoryFree:     "DirectMemory.free",
		MemTypeUnsafeAllocate:       "Unsafe.allocate",
		MemTypeUnsafeFree:           "Unsafe.free",
	}
	if name, ok := names[m]; ok {
		return name
	}
	return "unknown"
}

// JavaStackFrame Java栈帧
type JavaStackFrame struct {
	ClassName  string `json:"class_name"`
	MethodName string `json:"method_name"`
	FileName   string `json:"file_name"`
	LineNumber int    `json:"line_number"`
	Native     bool   `json:"native"`
}

// NativeStackFrame 本地栈帧
type NativeStackFrame struct {
	Address    uint64 `json:"address"`
	Symbol     string `json:"symbol"`
	Module     string `json:"module"`
	Offset     uint64 `json:"offset"`
}

// MemoryAllocation 内存分配记录
type MemoryAllocation struct {
	ID            uint64           `json:"id"`
	Timestamp     time.Time        `json:"timestamp"`
	PID           uint32           `json:"pid"`
	TID           uint32           `json:"tid"`
	MemoryType    MemoryType       `json:"memory_type"`
	Size          int64            `json:"size"`           // 正数分配，负数释放
	Address       uint64           `json:"address"`        // 内存地址
	JavaStack     []JavaStackFrame `json:"java_stack"`     // Java调用栈
	NativeStack   []NativeStackFrame `json:"native_stack"` // 本地调用栈
	IsLeakSuspect bool             `json:"is_leak_suspect"`
}

// AllocationStats 分配统计
type AllocationStats struct {
	MemoryType       MemoryType    `json:"memory_type"`
	TotalAllocated   uint64        `json:"total_allocated"`
	TotalFreed       uint64        `json:"total_freed"`
	CurrentUsed      uint64        `json:"current_used"`
	AllocationCount  uint64        `json:"allocation_count"`
	FreeCount        uint64        `json:"free_count"`
	AvgAllocationSize uint64       `json:"avg_allocation_size"`
	MaxAllocationSize uint64       `json:"max_allocation_size"`
}

// LeakReport 泄漏报告
type LeakReport struct {
	Timestamp       time.Time           `json:"timestamp"`
	PID             uint32              `json:"pid"`
	LeakType        MemoryType          `json:"leak_type"`
	LeakSize        uint64              `json:"leak_size"`
	GrowthRate      float64             `json:"growth_rate"`      // 字节/小时
	Confidence      float64             `json:"confidence"`       // 置信度 0-1
	SuspectStacks   []LeakSuspect       `json:"suspect_stacks"`
	UnfreedObjects  []*MemoryAllocation `json:"unfreed_objects"`
}

// LeakSuspect 泄漏嫌疑
type LeakSuspect struct {
	StackHash     string           `json:"stack_hash"`
	JavaStack     []JavaStackFrame `json:"java_stack"`
	AllocationCount uint64         `json:"allocation_count"`
	TotalSize     uint64           `json:"total_size"`
	UnfreedSize   uint64           `json:"unfreed_size"`
	LeakScore     float64          `json:"leak_score"`
}

// JVMMemReport Java内存报告
type JVMMemReport struct {
	Timestamp     time.Time                   `json:"timestamp"`
	PID           uint32                      `json:"pid"`
	StatsByType   map[MemoryType]*AllocationStats `json:"stats_by_type"`
	TotalUsed     uint64                      `json:"total_used"`
	TotalCapacity uint64                      `json:"total_capacity"`
	LeakReports   []*LeakReport               `json:"leak_reports"`
}

// ============================================================
// eBPF事件结构
// ============================================================

// JVMBPFEvent eBPF传递的事件结构
type JVMBPFEvent struct {
	Timestamp  uint64
	PID        uint32
	TID        uint32
	MemoryType uint32
	Size       int64
	Address    uint64
	StackID    int64
	ClassID    uint32 // Java类ID
	MethodID   uint32 // Java方法ID
	LineNum    uint32 // 行号
	Comm       [16]byte
}

// ============================================================
// Java内存分析器
// ============================================================

// JVMMemoryAnalyzer Java内存分析器
type JVMMemoryAnalyzer struct {
	config *JVMMemConfig

	// eBPF相关
	collection *ebpf.Collection
	links      []link.Link
	reader     *perf.Reader

	// 内存分配记录
	allocations     map[uint64]*MemoryAllocation // address -> allocation
	allocationsMu   sync.RWMutex
	allocationID    atomic.Uint64

	// 按调用栈聚合
	stackAllocations map[string][]*MemoryAllocation // stack_hash -> allocations
	stackMu          sync.RWMutex

	// 统计
	stats     map[MemoryType]*AllocationStats
	statsMu   sync.RWMutex

	// 泄漏检测历史
	history   []JVMMemSnapshot
	historyMu sync.RWMutex

	// Java符号表
	javaSymbols *JavaSymbolTable

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	running atomic.Bool

	// 报告
	reportChan chan *JVMMemReport

	// 目标进程
	targetPIDs map[uint32]bool
	pidMu      sync.RWMutex
}

// JVMMemSnapshot 内存快照
type JVMMemSnapshot struct {
	Timestamp time.Time
	PID       uint32
	Stats     map[MemoryType]*AllocationStats
}

// NewJVMMemoryAnalyzer 创建Java内存分析器
func NewJVMMemoryAnalyzer(cfg *JVMMemConfig) *JVMMemoryAnalyzer {
	if cfg == nil {
		cfg = DefaultJVMMemConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	a := &JVMMemoryAnalyzer{
		config:           cfg,
		allocations:      make(map[uint64]*MemoryAllocation),
		stackAllocations: make(map[string][]*MemoryAllocation),
		stats:            make(map[MemoryType]*AllocationStats),
		javaSymbols:      NewJavaSymbolTable(),
		targetPIDs:       make(map[uint32]bool),
		reportChan:       make(chan *JVMMemReport, 10),
		ctx:              ctx,
		cancel:           cancel,
	}

	return a
}

// Start 启动分析器
func (a *JVMMemoryAnalyzer) Start() error {
	if a.running.Load() {
		return fmt.Errorf("JVM memory analyzer already running")
	}

	// 加载eBPF程序
	if err := a.loadBPFProgram(); err != nil {
		return fmt.Errorf("failed to load BPF program: %w", err)
	}

	// 附加探针
	if err := a.attachProbes(); err != nil {
		return fmt.Errorf("failed to attach probes: %w", err)
	}

	// 初始化perf reader
	if err := a.initPerfReader(); err != nil {
		return fmt.Errorf("failed to init perf reader: %w", err)
	}

	a.running.Store(true)

	// 启动事件处理
	a.wg.Add(1)
	go a.processEvents()

	// 启动泄漏检测
	a.wg.Add(1)
	go a.leakDetectionLoop()

	// 启动报告生成
	a.wg.Add(1)
	go a.generateReports()

	return nil
}

// Stop 停止分析器
func (a *JVMMemoryAnalyzer) Stop() error {
	if !a.running.Load() {
		return nil
	}

	a.running.Store(false)
	a.cancel()

	if a.reader != nil {
		a.reader.Close()
	}

	for _, l := range a.links {
		if l != nil {
			l.Close()
		}
	}
	a.links = nil

	if a.collection != nil {
		a.collection.Close()
	}

	a.wg.Wait()
	close(a.reportChan)

	return nil
}

// loadBPFProgram 加载eBPF程序
func (a *JVMMemoryAnalyzer) loadBPFProgram() error {
	spec, err := ebpf.LoadCollectionSpec("bpf/jvm_memory.o")
	if err != nil {
		return a.loadInlineBPF()
	}

	collection, err := ebpf.NewCollection(spec)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	a.collection = collection
	return nil
}

// loadInlineBPF 加载内联eBPF程序
func (a *JVMMemoryAnalyzer) loadInlineBPF() error {
	return fmt.Errorf("inline BPF not implemented, please compile bpf/jvm_memory.bpf.c")
}

// attachProbes 附加探针
func (a *JVMMemoryAnalyzer) attachProbes() error {
	// 查找所有JVM进程
	jvmPIDs := a.findJavaProcesses()

	for _, pid := range jvmPIDs {
		// 附加ByteBuffer分配探针
		if a.config.TrackByteBuffer {
			a.attachByteBufferProbes(pid)
		}

		// 附加JNI内存探针
		if a.config.TrackJNIMemory {
			a.attachJNIProbes(pid)
		}

		// 附加DirectMemory探针
		if a.config.TrackDirectMemory {
			a.attachDirectMemoryProbes(pid)
		}
	}

	return nil
}

// attachByteBufferProbes 附加ByteBuffer探针
func (a *JVMMemoryAnalyzer) attachByteBufferProbes(pid uint32) {
	path := fmt.Sprintf("/proc/%d/exe", pid)

	// ByteBuffer.allocateDirect
	a.attachUSDT(pid, path, "java", "ByteBuffer_allocateDirect__entry", "bb_alloc_enter")
	a.attachUSDT(pid, path, "java", "ByteBuffer_allocateDirect__return", "bb_alloc_return")

	// DirectByteBuffer.cleaner/clean
	a.attachUSDT(pid, path, "java", "DirectByteBuffer_clean__entry", "bb_free_enter")
}

// attachJNIProbes 附加JNI内存探针
func (a *JVMMemoryAnalyzer) attachJNIProbes(pid uint32) {
	path := fmt.Sprintf("/proc/%d/exe", pid)

	// NewByteArray/NewDirectByteBuffer
	a.attachUSDT(pid, path, "java", "NewByteArray__entry", "jni_alloc_enter")
	a.attachUSDT(pid, path, "java", "NewDirectByteBuffer__entry", "jni_direct_alloc")

	// GetByteArrayElements/ReleaseByteArrayElements
	a.attachUSDT(pid, path, "java", "GetByteArrayElements__entry", "jni_get_elements")
	a.attachUSDT(pid, path, "java", "ReleaseByteArrayElements__entry", "jni_release_elements")
}

// attachDirectMemoryProbes 附加DirectMemory探针
func (a *JVMMemoryAnalyzer) attachDirectMemoryProbes(pid uint32) {
	path := fmt.Sprintf("/proc/%d/exe", pid)

	// Unsafe.allocateMemory
	a.attachUSDT(pid, path, "java", "Unsafe_allocateMemory__entry", "unsafe_alloc_enter")
	a.attachUSDT(pid, path, "java", "Unsafe_allocateMemory__return", "unsafe_alloc_return")

	// Unsafe.freeMemory
	a.attachUSDT(pid, path, "java", "Unsafe_freeMemory__entry", "unsafe_free_enter")
}

// attachUSDT 附加USDT探针
func (a *JVMMemoryAnalyzer) attachUSDT(pid uint32, path, provider, name, progName string) {
	if prog, ok := a.collection.Programs["usdt_"+progName]; ok {
		l, err := link.AttachUSDT(link.USDTOptions{
			PID:      int(pid),
			Path:     path,
			Provider: provider,
			Name:     name,
			Program:  prog,
		})
		if err == nil {
			a.links = append(a.links, l)
		}
	}
}

// initPerfReader 初始化perf reader
func (a *JVMMemoryAnalyzer) initPerfReader() error {
	eventsMap, ok := a.collection.Maps["jvm_mem_events"]
	if !ok {
		return errors.New("jvm_mem_events map not found")
	}

	reader, err := perf.NewReader(eventsMap, 65536)
	if err != nil {
		return fmt.Errorf("failed to create perf reader: %w", err)
	}

	a.reader = reader
	return nil
}

// processEvents 处理eBPF事件
func (a *JVMMemoryAnalyzer) processEvents() {
	defer a.wg.Done()

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			record, err := a.reader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					return
				}
				continue
			}

			if len(record.RawSample) > 0 {
				alloc := a.parseEvent(record.RawSample)
				if alloc != nil {
					a.processAllocation(alloc)
				}
			}
		}
	}
}

// parseEvent 解析eBPF事件
func (a *JVMMemoryAnalyzer) parseEvent(data []byte) *MemoryAllocation {
	if len(data) < int(unsafe.Sizeof(JVMBPFEvent{})) {
		return nil
	}

	var event JVMBPFEvent
	if err := binary.Read(
		unsafeReader{data: data[:unsafe.Sizeof(JVMBPFEvent{})]},
		binary.LittleEndian,
		&event,
	); err != nil {
		return nil
	}

	// 检查是否在目标PID列表中
	if !a.isTargetPID(event.PID) {
		return nil
	}

	// 采样检查
	if a.config.SampleRate < 100 {
		if fastRand()%100 >= uint32(a.config.SampleRate) {
			return nil
		}
	}

	alloc := &MemoryAllocation{
		ID:         a.allocationID.Add(1),
		Timestamp:  time.Unix(0, int64(event.Timestamp)),
		PID:        event.PID,
		TID:        event.TID,
		MemoryType: MemoryType(event.MemoryType),
		Size:       event.Size,
		Address:    event.Address,
	}

	// 解析Java栈
	if event.StackID >= 0 {
		alloc.JavaStack = a.resolveJavaStack(event.StackID, event.PID, event.ClassID, event.MethodID, event.LineNum)
	}

	return alloc
}

// resolveJavaStack 解析Java调用栈
func (a *JVMMemoryAnalyzer) resolveJavaStack(stackID int64, pid, classID, methodID, lineNum uint32) []JavaStackFrame {
	// 从eBPF map获取原始栈地址
	rawStack := a.getRawStack(stackID)
	if rawStack == nil {
		return nil
	}

	frames := make([]JavaStackFrame, 0, len(rawStack))

	for i, addr := range rawStack {
		if addr == 0 {
			continue
		}

		frame := &JavaStackFrame{}

		// 第一帧使用USDT传递的符号信息
		if i == 0 && classID != 0 && methodID != 0 {
			frame.ClassName = a.javaSymbols.GetClassName(pid, classID)
			frame.MethodName = a.javaSymbols.GetMethodName(pid, methodID)
			frame.LineNumber = int(lineNum)
		} else {
			// 从符号表解析
			sym := a.javaSymbols.Resolve(pid, addr)
			if sym != nil {
				frame.ClassName = sym.ClassName
				frame.MethodName = sym.MethodName
				frame.FileName = sym.FileName
				frame.LineNumber = sym.LineNumber
			}
		}

		// 如果符号解析失败，使用地址
		if frame.ClassName == "" {
			frame.ClassName = fmt.Sprintf("0x%x", addr)
			frame.Native = true
		}

		frames = append(frames, *frame)
	}

	return frames
}

// getRawStack 获取原始栈地址
func (a *JVMMemoryAnalyzer) getRawStack(stackID int64) []uint64 {
	if stackID < 0 {
		return nil
	}

	stackMap, ok := a.collection.Maps["stack_traces"]
	if !ok {
		return nil
	}

	key := uint32(stackID)
	value := make([]uint64, a.config.MaxStackDepth)
	
	err := stackMap.Lookup(&key, &value)
	if err != nil {
		return nil
	}

	// 找到实际长度
	length := 0
	for i, addr := range value {
		if addr == 0 {
			break
		}
		length = i + 1
	}

	return value[:length]
}

// processAllocation 处理内存分配记录
func (a *JVMMemoryAnalyzer) processAllocation(alloc *MemoryAllocation) {
	a.allocationsMu.Lock()
	defer a.allocationsMu.Unlock()

	// 更新统计
	a.updateStats(alloc)

	if alloc.Size > 0 {
		// 分配
		a.allocations[alloc.Address] = alloc

		// 按栈聚合
		stackHash := a.hashJavaStack(alloc.JavaStack)
		a.stackMu.Lock()
		a.stackAllocations[stackHash] = append(a.stackAllocations[stackHash], alloc)
		a.stackMu.Unlock()
	} else {
		// 释放
		if oldAlloc, ok := a.allocations[alloc.Address]; ok {
			oldAlloc.Size = -oldAlloc.Size // 标记为已释放
			delete(a.allocations, alloc.Address)
		}
	}
}

// updateStats 更新统计
func (a *JVMMemoryAnalyzer) updateStats(alloc *MemoryAllocation) {
	a.statsMu.Lock()
	defer a.statsMu.Unlock()

	stats, ok := a.stats[alloc.MemoryType]
	if !ok {
		stats = &AllocationStats{MemoryType: alloc.MemoryType}
		a.stats[alloc.MemoryType] = stats
	}

	if alloc.Size > 0 {
		stats.TotalAllocated += uint64(alloc.Size)
		stats.CurrentUsed += uint64(alloc.Size)
		stats.AllocationCount++
		if uint64(alloc.Size) > stats.MaxAllocationSize {
			stats.MaxAllocationSize = uint64(alloc.Size)
		}
	} else {
		size := uint64(-alloc.Size)
		stats.TotalFreed += size
		if stats.CurrentUsed >= size {
			stats.CurrentUsed -= size
		}
		stats.FreeCount++
	}

	if stats.AllocationCount > 0 {
		stats.AvgAllocationSize = stats.TotalAllocated / stats.AllocationCount
	}
}

// leakDetectionLoop 泄漏检测循环
func (a *JVMMemoryAnalyzer) leakDetectionLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.LeakCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.detectLeaks()
		}
	}
}

// detectLeaks 检测内存泄漏
func (a *JVMMemoryAnalyzer) detectLeaks() {
	// 保存当前快照
	snapshot := a.takeSnapshot()

	a.historyMu.Lock()
	a.history = append(a.history, snapshot)

	// 保留最近24小时的历史
	cutoff := time.Now().Add(-24 * time.Hour)
	newHistory := make([]JVMMemSnapshot, 0, len(a.history))
	for _, h := range a.history {
		if h.Timestamp.After(cutoff) {
			newHistory = append(newHistory, h)
		}
	}
	a.history = newHistory
	a.historyMu.Unlock()

	// 分析泄漏
	if len(a.history) < 2 {
		return
	}

	// 计算增长率
	for memType, stats := range snapshot.Stats {
		if stats.CurrentUsed < a.config.MinLeakSize {
			continue
		}

		growthRate := a.calculateGrowthRate(memType)
		if growthRate > a.config.LeakThreshold {
			// 发现疑似泄漏
			leakReport := a.buildLeakReport(memType, growthRate)
			_ = leakReport
		}
	}
}

// takeSnapshot 获取内存快照
func (a *JVMMemoryAnalyzer) takeSnapshot() JVMMemSnapshot {
	a.statsMu.RLock()
	defer a.statsMu.RUnlock()

	statsCopy := make(map[MemoryType]*AllocationStats)
	for k, v := range a.stats {
		cp := *v
		statsCopy[k] = &cp
	}

	return JVMMemSnapshot{
		Timestamp: time.Now(),
		Stats:     statsCopy,
	}
}

// calculateGrowthRate 计算增长率（字节/小时）
func (a *JVMMemoryAnalyzer) calculateGrowthRate(memType MemoryType) float64 {
	if len(a.history) < 2 {
		return 0
	}

	first := a.history[0]
	last := a.history[len(a.history)-1]

	duration := last.Timestamp.Sub(first.Timestamp).Hours()
	if duration <= 0 {
		return 0
	}

	firstStats, ok1 := first.Stats[memType]
	lastStats, ok2 := last.Stats[memType]
	if !ok1 || !ok2 {
		return 0
	}

	growth := float64(lastStats.CurrentUsed - firstStats.CurrentUsed)
	return growth / duration
}

// buildLeakReport 构建泄漏报告
func (a *JVMMemoryAnalyzer) buildLeakReport(memType MemoryType, growthRate float64) *LeakReport {
	a.allocationsMu.RLock()
	defer a.allocationsMu.RUnlock()

	a.statsMu.RLock()
	stats := a.stats[memType]
	a.statsMu.RUnlock()

	report := &LeakReport{
		Timestamp:  time.Now(),
		LeakType:   memType,
		LeakSize:   stats.CurrentUsed,
		GrowthRate: growthRate,
		Confidence: a.calculateLeakConfidence(memType, growthRate),
	}

	// 找出嫌疑调用栈
	a.stackMu.RLock()
	suspects := make([]*LeakSuspect, 0)
	for stackHash, allocs := range a.stackAllocations {
		var unfreedSize uint64
		var allocCount uint64
		for _, alloc := range allocs {
			if alloc.MemoryType == memType && alloc.Size > 0 {
				allocCount++
				unfreedSize += uint64(alloc.Size)
			}
		}

		if unfreedSize > a.config.MinLeakSize/10 {
			suspect := &LeakSuspect{
				StackHash:       stackHash,
				AllocationCount: allocCount,
				TotalSize:       unfreedSize,
				UnfreedSize:     unfreedSize,
				LeakScore:       float64(unfreedSize) / float64(stats.CurrentUsed),
			}
			if len(allocs) > 0 {
				suspect.JavaStack = allocs[0].JavaStack
			}
			suspects = append(suspects, suspect)
		}
	}
	a.stackMu.RUnlock()

	// 按泄漏分数排序
	sort.Slice(suspects, func(i, j int) bool {
		return suspects[i].LeakScore > suspects[j].LeakScore
	})

	// 取前10个嫌疑栈
	if len(suspects) > 10 {
		suspects = suspects[:10]
	}

	for _, s := range suspects {
		report.SuspectStacks = append(report.SuspectStacks, *s)
	}

	return report
}

// calculateLeakConfidence 计算泄漏置信度
func (a *JVMMemoryAnalyzer) calculateLeakConfidence(memType MemoryType, growthRate float64) float64 {
	// 基于多个因素计算置信度
	confidence := 0.0

	// 增长率因素
	if growthRate > a.config.LeakThreshold*2 {
		confidence += 0.4
	} else if growthRate > a.config.LeakThreshold {
		confidence += 0.3
	}

	// 历史一致性
	a.historyMu.RLock()
	if len(a.history) >= 3 {
		confidence += 0.3
	}
	a.historyMu.RUnlock()

	// 未释放对象比例
	a.statsMu.RLock()
	stats := a.stats[memType]
	if stats.AllocationCount > 0 {
		unfreedRatio := float64(stats.AllocationCount-stats.FreeCount) / float64(stats.AllocationCount)
		if unfreedRatio > 0.8 {
			confidence += 0.3
		}
	}
	a.statsMu.RUnlock()

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// generateReports 生成报告
func (a *JVMMemoryAnalyzer) generateReports() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			report := a.buildReport()
			select {
			case a.reportChan <- report:
			default:
			}
		}
	}
}

// buildReport 构建报告
func (a *JVMMemoryAnalyzer) buildReport() *JVMMemReport {
	a.statsMu.RLock()
	statsCopy := make(map[MemoryType]*AllocationStats)
	for k, v := range a.stats {
		cp := *v
		statsCopy[k] = &cp
	}
	a.statsMu.RUnlock()

	var totalUsed uint64
	for _, s := range statsCopy {
		totalUsed += s.CurrentUsed
	}

	return &JVMMemReport{
		Timestamp:   time.Now(),
		StatsByType: statsCopy,
		TotalUsed:   totalUsed,
	}
}

// ============================================================
// 公共API
// ============================================================

// AttachPID 动态附加到JVM进程
func (a *JVMMemoryAnalyzer) AttachPID(pid uint32) error {
	a.pidMu.Lock()
	defer a.pidMu.Unlock()

	a.targetPIDs[pid] = true

	// 加载Java符号表
	a.javaSymbols.Load(pid)

	// 附加探针
	path := fmt.Sprintf("/proc/%d/exe", pid)
	
	if a.config.TrackByteBuffer {
		a.attachByteBufferProbes(pid)
	}
	if a.config.TrackJNIMemory {
		a.attachJNIProbes(pid)
	}
	if a.config.TrackDirectMemory {
		a.attachDirectMemoryProbes(pid)
	}

	_ = path
	return nil
}

// DetachPID 动态分离JVM进程
func (a *JVMMemoryAnalyzer) DetachPID(pid uint32) error {
	a.pidMu.Lock()
	defer a.pidMu.Unlock()

	delete(a.targetPIDs, pid)
	a.javaSymbols.Unload(pid)

	return nil
}

// GetReport 获取当前报告
func (a *JVMMemoryAnalyzer) GetReport() *JVMMemReport {
	return a.buildReport()
}

// Reports 返回报告通道
func (a *JVMMemoryAnalyzer) Reports() <-chan *JVMMemReport {
	return a.reportChan
}

// GetAllocations 获取分配记录
func (a *JVMMemoryAnalyzer) GetAllocations() []*MemoryAllocation {
	a.allocationsMu.RLock()
	defer a.allocationsMu.RUnlock()

	allocs := make([]*MemoryAllocation, 0, len(a.allocations))
	for _, alloc := range a.allocations {
		allocs = append(allocs, alloc)
	}
	return allocs
}

// GetLeakSuspects 获取泄漏嫌疑
func (a *JVMMemoryAnalyzer) GetLeakSuspects(memType MemoryType) []*LeakSuspect {
	a.stackMu.RLock()
	defer a.stackMu.RUnlock()

	suspects := make([]*LeakSuspect, 0)
	for stackHash, allocs := range a.stackAllocations {
		var unfreedSize uint64
		var allocCount uint64
		var sampleAlloc *MemoryAllocation

		for _, alloc := range allocs {
			if alloc.MemoryType == memType && alloc.Size > 0 {
				allocCount++
				unfreedSize += uint64(alloc.Size)
				if sampleAlloc == nil {
					sampleAlloc = alloc
				}
			}
		}

		if unfreedSize > 0 {
			suspect := &LeakSuspect{
				StackHash:       stackHash,
				AllocationCount: allocCount,
				TotalSize:       unfreedSize,
				UnfreedSize:     unfreedSize,
			}
			if sampleAlloc != nil {
				suspect.JavaStack = sampleAlloc.JavaStack
			}
			suspects = append(suspects, suspect)
		}
	}

	// 按未释放大小排序
	sort.Slice(suspects, func(i, j int) bool {
		return suspects[i].UnfreedSize > suspects[j].UnfreedSize
	})

	return suspects
}

// ClearData 清除数据
func (a *JVMMemoryAnalyzer) ClearData() {
	a.allocationsMu.Lock()
	a.allocations = make(map[uint64]*MemoryAllocation)
	a.allocationsMu.Unlock()

	a.stackMu.Lock()
	a.stackAllocations = make(map[string][]*MemoryAllocation)
	a.stackMu.Unlock()

	a.statsMu.Lock()
	a.stats = make(map[MemoryType]*AllocationStats)
	a.statsMu.Unlock()

	a.historyMu.Lock()
	a.history = make([]JVMMemSnapshot, 0)
	a.historyMu.Unlock()
}

// ============================================================
// 辅助函数
// ============================================================

func (a *JVMMemoryAnalyzer) isTargetPID(pid uint32) bool {
	a.pidMu.RLock()
	defer a.pidMu.RUnlock()

	if len(a.targetPIDs) == 0 {
		return true
	}

	return a.targetPIDs[pid]
}

func (a *JVMMemoryAnalyzer) findJavaProcesses() []uint32 {
	var pids []uint32

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return pids
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.ParseUint(entry.Name(), 10, 32)
		if err != nil {
			continue
		}

		comm, err := os.ReadFile(fmt.Sprintf("/proc/%s/comm", entry.Name()))
		if err != nil {
			continue
		}

		if strings.Contains(string(comm), "java") {
			pids = append(pids, uint32(pid))
		}
	}

	return pids
}

func (a *JVMMemoryAnalyzer) hashJavaStack(frames []JavaStackFrame) string {
	if len(frames) == 0 {
		return "empty"
	}

	var sb strings.Builder
	for i, frame := range frames {
		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteString(frame.ClassName)
		sb.WriteByte('.')
		sb.WriteString(frame.MethodName)
	}
	return sb.String()
}

func fastRand() uint32 {
	// 简单快速随机数
	return uint32(time.Now().UnixNano())
}

// unsafeReader 用于从字节切片读取
type unsafeReader struct {
	data []byte
	pos  int
}

func (r unsafeReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, errors.New("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
