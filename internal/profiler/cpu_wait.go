// Package profiler 提供CPU等待瓶颈分析功能
// 支持C/C++/Golang/Java程序的无侵入、动态采样分析
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package profiler

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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

// CPUWaitConfig CPU等待分析配置
type CPUWaitConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	SampleRate        int           `yaml:"sample_rate" json:"sample_rate"`           // 采样率 (Hz)
	MaxSamplesPerSec  int           `yaml:"max_samples_per_sec" json:"max_samples_per_sec"` // 每秒最大采样数
	TargetLanguages   []string      `yaml:"target_languages" json:"target_languages"`   // 目标语言: c,cpp,go,java
	TargetPIDs        []uint32      `yaml:"target_pids" json:"target_pids"`           // 目标进程ID，空表示所有
	MinBlockTime      time.Duration `yaml:"min_block_time" json:"min_block_time"`     // 最小阻塞时间
	MaxStackDepth     int           `yaml:"max_stack_depth" json:"max_stack_depth"`   // 最大栈深度
	SymbolResolution  bool          `yaml:"symbol_resolution" json:"symbol_resolution"` // 符号解析
	ReportInterval    time.Duration `yaml:"report_interval" json:"report_interval"`   // 报告间隔
}

func DefaultCPUWaitConfig() *CPUWaitConfig {
	return &CPUWaitConfig{
		Enabled:          true,
		SampleRate:       99, // 99Hz避免与定时器对齐
		MaxSamplesPerSec: 1000,
		TargetLanguages:  []string{"c", "cpp", "go", "java"},
		TargetPIDs:       []uint32{},
		MinBlockTime:     1 * time.Millisecond,
		MaxStackDepth:    128,
		SymbolResolution: true,
		ReportInterval:   60 * time.Second,
	}
}

// ============================================================
// 数据模型
// ============================================================

// LanguageType 编程语言类型
type LanguageType int

const (
	LangUnknown LanguageType = iota
	LangC
	LangCPP
	LangGo
	LangJava
)

func (l LanguageType) String() string {
	switch l {
	case LangC:
		return "C"
	case LangCPP:
		return "C++"
	case LangGo:
		return "Go"
	case LangJava:
		return "Java"
	default:
		return "Unknown"
	}
}

// WaitReason 等待原因
type WaitReason int

const (
	WaitUnknown WaitReason = iota
	WaitFutex           // futex等待
	WaitIO              // IO等待
	WaitNetwork         // 网络等待
	WaitLock            // 锁等待
	WaitCondVar         // 条件变量等待
	WaitSleep           // 睡眠
	WaitPark            // goroutine park
	WaitMonitor         // Java monitor
	WaitParkNanos       // LockSupport.parkNanos
)

func (w WaitReason) String() string {
	names := map[WaitReason]string{
		WaitFutex:       "futex",
		WaitIO:          "io",
		WaitNetwork:     "network",
		WaitLock:        "lock",
		WaitCondVar:     "condvar",
		WaitSleep:       "sleep",
		WaitPark:        "park",
		WaitMonitor:     "monitor",
		WaitParkNanos:   "park_nanos",
	}
	if name, ok := names[w]; ok {
		return name
	}
	return "unknown"
}

// StackFrame 栈帧信息
type StackFrame struct {
	Address    uint64 `json:"address"`
	Symbol     string `json:"symbol"`
	Module     string `json:"module"`
	Offset     uint64 `json:"offset"`
	SourceFile string `json:"source_file,omitempty"`
	LineNumber int    `json:"line_number,omitempty"`
}

// CPUWaitSample CPU等待采样
type CPUWaitSample struct {
	Timestamp    time.Time      `json:"timestamp"`
	PID          uint32         `json:"pid"`
	TID          uint32         `json:"tid"`
	Language     LanguageType   `json:"language"`
	Comm         string         `json:"comm"`
	WaitReason   WaitReason     `json:"wait_reason"`
	WaitDuration time.Duration  `json:"wait_duration"`
	StackTrace   []StackFrame   `json:"stack_trace"`
	CPU          uint32         `json:"cpu"`
}

// CPUWaitReport CPU等待分析报告
type CPUWaitReport struct {
	StartTime       time.Time                `json:"start_time"`
	EndTime         time.Time                `json:"end_time"`
	TotalSamples    uint64                   `json:"total_samples"`
	SamplesByLang   map[LanguageType]uint64  `json:"samples_by_lang"`
	SamplesByReason map[WaitReason]uint64    `json:"samples_by_reason"`
	Hotspots        []WaitHotspot            `json:"hotspots"`
}

// WaitHotspot 等待热点
type WaitHotspot struct {
	StackHash    string         `json:"stack_hash"`
	StackTrace   []StackFrame   `json:"stack_trace"`
	Count        uint64         `json:"count"`
	TotalWait    time.Duration  `json:"total_wait"`
	AvgWait      time.Duration  `json:"avg_wait"`
	Language     LanguageType   `json:"language"`
	WaitReason   WaitReason     `json:"wait_reason"`
}

// ============================================================
// eBPF事件结构
// ============================================================

// CPUBPFEvent eBPF传递的事件结构
type CPUBPFEvent struct {
	Timestamp   uint64
	PID         uint32
	TID         uint32
	CPU         uint32
	Language    uint32
	WaitReason  uint32
	WaitTimeNs  uint64
	StackID     int64
	Comm        [16]byte
}

// ============================================================
// CPU等待分析器
// ============================================================

// CPUWaitProfiler CPU等待分析器
type CPUWaitProfiler struct {
	config *CPUWaitConfig

	// eBPF相关
	collection *ebpf.Collection
	links      []link.Link
	reader     *perf.Reader

	// 采样数据
	samples     []*CPUWaitSample
	samplesMu   sync.RWMutex
	sampleCount atomic.Uint64

	// 栈跟踪存储
	stackTraces map[int64][]uint64
	stackMu     sync.RWMutex

	// 符号解析器
	symbolizers map[LanguageType]Symbolizer

	// 动态采样控制
	sampleRate    atomic.Int32
	dynamicAdjust bool

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	running atomic.Bool

	// 报告
	reportChan chan *CPUWaitReport

	// 目标进程
	targetPIDs map[uint32]bool
	pidMu      sync.RWMutex
}

// Symbolizer 符号解析接口
type Symbolizer interface {
	Resolve(addr uint64, pid uint32) (*StackFrame, error)
	LoadSymbols(pid uint32) error
	UnloadSymbols(pid uint32)
}

// NewCPUWaitProfiler 创建CPU等待分析器
func NewCPUWaitProfiler(cfg *CPUWaitConfig) *CPUWaitProfiler {
	if cfg == nil {
		cfg = DefaultCPUWaitConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &CPUWaitProfiler{
		config:      cfg,
		samples:     make([]*CPUWaitSample, 0, 10000),
		stackTraces: make(map[int64][]uint64),
		symbolizers: make(map[LanguageType]Symbolizer),
		targetPIDs:  make(map[uint32]bool),
		reportChan:  make(chan *CPUWaitReport, 10),
		ctx:         ctx,
		cancel:      cancel,
	}

	p.sampleRate.Store(int32(cfg.SampleRate))

	// 初始化符号解析器
	p.initSymbolizers()

	return p
}

// initSymbolizers 初始化符号解析器
func (p *CPUWaitProfiler) initSymbolizers() {
	for _, lang := range p.config.TargetLanguages {
		switch lang {
		case "c", "cpp":
			p.symbolizers[LangC] = NewNativeSymbolizer()
			p.symbolizers[LangCPP] = p.symbolizers[LangC]
		case "go":
			p.symbolizers[LangGo] = NewGoSymbolizer()
		case "java":
			p.symbolizers[LangJava] = NewJavaSymbolizer()
		}
	}
}

// Start 启动分析器
func (p *CPUWaitProfiler) Start() error {
	if p.running.Load() {
		return fmt.Errorf("CPU wait profiler already running")
	}

	// 加载eBPF程序
	if err := p.loadBPFProgram(); err != nil {
		return fmt.Errorf("failed to load BPF program: %w", err)
	}

	// 附加探针
	if err := p.attachProbes(); err != nil {
		return fmt.Errorf("failed to attach probes: %w", err)
	}

	// 初始化perf reader
	if err := p.initPerfReader(); err != nil {
		return fmt.Errorf("failed to init perf reader: %w", err)
	}

	p.running.Store(true)

	// 启动事件处理
	p.wg.Add(1)
	go p.processEvents()

	// 启动动态采样调整
	if p.dynamicAdjust {
		p.wg.Add(1)
		go p.adjustSampleRate()
	}

	// 启动报告生成
	p.wg.Add(1)
	go p.generateReports()

	return nil
}

// Stop 停止分析器
func (p *CPUWaitProfiler) Stop() error {
	if !p.running.Load() {
		return nil
	}

	p.running.Store(false)
	p.cancel()

	// 关闭reader
	if p.reader != nil {
		p.reader.Close()
	}

	// 分离链接
	for _, l := range p.links {
		if l != nil {
			l.Close()
		}
	}
	p.links = nil

	// 关闭collection
	if p.collection != nil {
		p.collection.Close()
	}

	p.wg.Wait()
	close(p.reportChan)

	return nil
}

// loadBPFProgram 加载eBPF程序
func (p *CPUWaitProfiler) loadBPFProgram() error {
	spec, err := ebpf.LoadCollectionSpec("bpf/cpu_wait.o")
	if err != nil {
		// 如果预编译文件不存在，使用内联加载
		return p.loadInlineBPF()
	}

	collection, err := ebpf.NewCollection(spec)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	p.collection = collection
	return nil
}

// loadInlineBPF 加载内联eBPF程序
func (p *CPUWaitProfiler) loadInlineBPF() error {
	// 实际项目中使用go:generate bpf2go生成
	// 这里提供框架代码
	return fmt.Errorf("inline BPF not implemented, please compile bpf/cpu_wait.bpf.c")
}

// attachProbes 附加探针
func (p *CPUWaitProfiler) attachProbes() error {
	// 附加sched_switch探针 - 检测上下文切换
	if prog, ok := p.collection.Programs["trace_sched_switch"]; ok {
		l, err := link.Tracepoint("sched", "sched_switch", prog, nil)
		if err != nil {
			return fmt.Errorf("failed to attach sched_switch: %w", err)
		}
		p.links = append(p.links, l)
	}

	// 附加futex等待探针
	if prog, ok := p.collection.Programs["kprobe_futex_wait"]; ok {
		l, err := link.Kprobe("do_futex", prog, nil)
		if err != nil {
			// 某些内核可能没有此符号，忽略错误
			_ = err
		} else {
			p.links = append(p.links, l)
		}
	}

	// 附加io等待探针
	if prog, ok := p.collection.Programs["kprobe_io_schedule"]; ok {
		l, err := link.Kprobe("io_schedule", prog, nil)
		if err == nil {
			p.links = append(p.links, l)
		}
	}

	// 附加Go运行时探针（如果检测到Go程序）
	if p.isLanguageEnabled("go") {
		p.attachGoProbes()
	}

	// 附加JVM探针（如果检测到Java程序）
	if p.isLanguageEnabled("java") {
		p.attachJavaProbes()
	}

	return nil
}

// attachGoProbes 附加Go运行时探针
func (p *CPUWaitProfiler) attachGoProbes() {
	// 附加runtime.futexsleep
	if prog, ok := p.collection.Programs["kprobe_go_futexsleep"]; ok {
		l, err := link.Kprobe("runtime.futexsleep", prog, nil)
		if err == nil {
			p.links = append(p.links, l)
		}
	}

	// 附加runtime.park_m
	if prog, ok := p.collection.Programs["kprobe_go_park"]; ok {
		l, err := link.Kprobe("runtime.park_m", prog, nil)
		if err == nil {
			p.links = append(p.links, l)
		}
	}

	// 附加runtime.lock
	if prog, ok := p.collection.Programs["kprobe_go_lock"]; ok {
		l, err := link.Kprobe("runtime.lock", prog, nil)
		if err == nil {
			p.links = append(p.links, l)
		}
	}
}

// attachJavaProbes 附加JVM探针
func (p *CPUWaitProfiler) attachJavaProbes() {
	// 使用USDT探针附加JVM内部事件
	// 需要JVM开启-XX:+DTraceMethodProbes

	// 查找JVM进程
	jvmPIDs := p.findJavaProcesses()
	for _, pid := range jvmPIDs {
		// 附加JVM的monitor enter/exit探针
		p.attachJVMUSDT(pid, "monitor__enter", "monitor_enter")
		p.attachJVMUSDT(pid, "monitor__exit", "monitor_exit")
		p.attachJVMUSDT(pid, "thread__sleep__begin", "thread_sleep_begin")
		p.attachJVMUSDT(pid, "thread__sleep__end", "thread_sleep_end")
		p.attachJVMUSDT(pid, "thread__park__begin", "thread_park_begin")
		p.attachJVMUSDT(pid, "thread__park__end", "thread_park_end")
	}
}

// attachJVMUSDT 附加JVM USDT探针
func (p *CPUWaitProfiler) attachJVMUSDT(pid uint32, provider, name string) {
	path := fmt.Sprintf("/proc/%d/exe", pid)
	
	if prog, ok := p.collection.Programs["usdt_"+name]; ok {
		l, err := link.AttachUSDT(link.USDTOptions{
			PID:      int(pid),
			Path:     path,
			Provider: provider,
			Name:     name,
			Program:  prog,
		})
		if err == nil {
			p.links = append(p.links, l)
		}
	}
}

// initPerfReader 初始化perf reader
func (p *CPUWaitProfiler) initPerfReader() error {
	eventsMap, ok := p.collection.Maps["cpu_wait_events"]
	if !ok {
		return errors.New("cpu_wait_events map not found")
	}

	reader, err := perf.NewReader(eventsMap, 65536)
	if err != nil {
		return fmt.Errorf("failed to create perf reader: %w", err)
	}

	p.reader = reader
	return nil
}

// processEvents 处理eBPF事件
func (p *CPUWaitProfiler) processEvents() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			record, err := p.reader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					return
				}
				continue
			}

			if len(record.RawSample) > 0 {
				sample := p.parseEvent(record.RawSample)
				if sample != nil {
					p.storeSample(sample)
				}
			}
		}
	}
}

// parseEvent 解析eBPF事件
func (p *CPUWaitProfiler) parseEvent(data []byte) *CPUWaitSample {
	if len(data) < int(unsafe.Sizeof(CPUBPFEvent{})) {
		return nil
	}

	var event CPUBPFEvent
	if err := binary.Read(
		unsafeReader{data: data[:unsafe.Sizeof(CPUBPFEvent{})]},
		binary.LittleEndian,
		&event,
	); err != nil {
		return nil
	}

	// 检查是否在目标PID列表中
	if !p.isTargetPID(event.PID) {
		return nil
	}

	sample := &CPUWaitSample{
		Timestamp:    time.Unix(0, int64(event.Timestamp)),
		PID:          event.PID,
		TID:          event.TID,
		CPU:          event.CPU,
		Language:     LanguageType(event.Language),
		WaitReason:   WaitReason(event.WaitReason),
		WaitDuration: time.Duration(event.WaitTimeNs) * time.Nanosecond,
		Comm:         string(bytesTrimNull(event.Comm[:])),
	}

	// 解析栈跟踪
	if event.StackID >= 0 {
		sample.StackTrace = p.resolveStack(event.StackID, event.PID, sample.Language)
	}

	return sample
}

// resolveStack 解析栈跟踪
func (p *CPUWaitProfiler) resolveStack(stackID int64, pid uint32, lang LanguageType) []StackFrame {
	p.stackMu.RLock()
	addrs, ok := p.stackTraces[stackID]
	p.stackMu.RUnlock()

	if !ok {
		return nil
	}

	frames := make([]StackFrame, 0, len(addrs))
	
	// 获取符号解析器
	symbolizer, ok := p.symbolizers[lang]
	if !ok {
		symbolizer = p.symbolizers[LangC]
	}

	for _, addr := range addrs {
		if addr == 0 {
			continue
		}

		frame, err := symbolizer.Resolve(addr, pid)
		if err != nil {
			frame = &StackFrame{
				Address: addr,
				Symbol:  fmt.Sprintf("0x%x", addr),
			}
		}
		
		if frame != nil {
			frames = append(frames, *frame)
		}
	}

	return frames
}

// storeSample 存储采样
func (p *CPUWaitProfiler) storeSample(sample *CPUWaitSample) {
	p.samplesMu.Lock()
	defer p.samplesMu.Unlock()

	// 限制样本数量
	if len(p.samples) >= 100000 {
		// 移除最老的10%
		p.samples = p.samples[10000:]
	}

	p.samples = append(p.samples, sample)
	p.sampleCount.Add(1)
}

// adjustSampleRate 动态调整采样率
func (p *CPUWaitProfiler) adjustSampleRate() {
	defer p.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.doAdjustSampleRate()
		}
	}
}

// doAdjustSampleRate 执行采样率调整
func (p *CPUWaitProfiler) doAdjustSampleRate() {
	currentRate := p.sampleRate.Load()
	sampleCount := p.sampleCount.Swap(0)

	// 计算当前采样率
	currentSamplesPerSec := int32(sampleCount / 10)

	if currentSamplesPerSec > int32(p.config.MaxSamplesPerSec) {
		// 采样过多，降低采样率
		newRate := currentRate * 80 / 100
		if newRate < 1 {
			newRate = 1
		}
		p.sampleRate.Store(newRate)
		p.updateBPFSampleRate(newRate)
	} else if currentSamplesPerSec < int32(p.config.MaxSamplesPerSec/2) {
		// 采样过少，提高采样率
		newRate := currentRate * 120 / 100
		if newRate > int32(p.config.SampleRate*2) {
			newRate = int32(p.config.SampleRate * 2)
		}
		p.sampleRate.Store(newRate)
		p.updateBPFSampleRate(newRate)
	}
}

// updateBPFSampleRate 更新eBPF采样率
func (p *CPUWaitProfiler) updateBPFSampleRate(rate int32) {
	if rateMap, ok := p.collection.Maps["sample_rate"]; ok {
		key := uint32(0)
		value := uint32(rate)
		rateMap.Update(&key, &value, ebpf.UpdateAny)
	}
}

// generateReports 生成报告
func (p *CPUWaitProfiler) generateReports() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			report := p.buildReport()
			select {
			case p.reportChan <- report:
			default:
			}
		}
	}
}

// buildReport 构建报告
func (p *CPUWaitProfiler) buildReport() *CPUWaitReport {
	p.samplesMu.RLock()
	samples := make([]*CPUWaitSample, len(p.samples))
	copy(samples, p.samples)
	p.samplesMu.RUnlock()

	report := &CPUWaitReport{
		StartTime:       time.Now().Add(-p.config.ReportInterval),
		EndTime:         time.Now(),
		TotalSamples:    uint64(len(samples)),
		SamplesByLang:   make(map[LanguageType]uint64),
		SamplesByReason: make(map[WaitReason]uint64),
	}

	// 统计热点
	hotspotMap := make(map[string]*WaitHotspot)

	for _, sample := range samples {
		report.SamplesByLang[sample.Language]++
		report.SamplesByReason[sample.WaitReason]++

		// 计算栈哈希
		stackHash := p.hashStack(sample.StackTrace)
		
		if hotspot, ok := hotspotMap[stackHash]; ok {
			hotspot.Count++
			hotspot.TotalWait += sample.WaitDuration
		} else {
			hotspotMap[stackHash] = &WaitHotspot{
				StackHash:  stackHash,
				StackTrace: sample.StackTrace,
				Count:      1,
				TotalWait:  sample.WaitDuration,
				Language:   sample.Language,
				WaitReason: sample.WaitReason,
			}
		}
	}

	// 转换为切片并计算平均值
	report.Hotspots = make([]WaitHotspot, 0, len(hotspotMap))
	for _, hotspot := range hotspotMap {
		if hotspot.Count > 0 {
			hotspot.AvgWait = hotspot.TotalWait / time.Duration(hotspot.Count)
		}
		report.Hotspots = append(report.Hotspots, *hotspot)
	}

	// 按等待时间排序
	p.sortHotspots(report.Hotspots)

	return report
}

// hashStack 计算栈哈希
func (p *CPUWaitProfiler) hashStack(frames []StackFrame) string {
	if len(frames) == 0 {
		return "empty"
	}

	var sb strings.Builder
	for i, frame := range frames {
		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteString(fmt.Sprintf("%x", frame.Address))
	}
	return sb.String()
}

// sortHotspots 排序热点
func (p *CPUWaitProfiler) sortHotspots(hotspots []WaitHotspot) {
	// 简单的冒泡排序，按等待时间降序
	for i := 0; i < len(hotspots); i++ {
		for j := i + 1; j < len(hotspots); j++ {
			if hotspots[j].TotalWait > hotspots[i].TotalWait {
				hotspots[i], hotspots[j] = hotspots[j], hotspots[i]
			}
		}
	}
}

// ============================================================
// 公共API
// ============================================================

// AttachPID 动态附加到进程
func (p *CPUWaitProfiler) AttachPID(pid uint32) error {
	p.pidMu.Lock()
	defer p.pidMu.Unlock()

	p.targetPIDs[pid] = true

	// 加载进程符号
	lang := p.detectLanguage(pid)
	if symbolizer, ok := p.symbolizers[lang]; ok {
		symbolizer.LoadSymbols(pid)
	}

	// 如果是Java进程，附加USDT探针
	if lang == LangJava {
		p.attachJVMUSDT(pid, "monitor__enter", "monitor_enter")
		p.attachJVMUSDT(pid, "monitor__exit", "monitor_exit")
	}

	return nil
}

// DetachPID 动态分离进程
func (p *CPUWaitProfiler) DetachPID(pid uint32) error {
	p.pidMu.Lock()
	defer p.pidMu.Unlock()

	delete(p.targetPIDs, pid)

	// 卸载进程符号
	for _, symbolizer := range p.symbolizers {
		symbolizer.UnloadSymbols(pid)
	}

	return nil
}

// GetReport 获取报告
func (p *CPUWaitProfiler) GetReport() *CPUWaitReport {
	return p.buildReport()
}

// Reports 返回报告通道
func (p *CPUWaitProfiler) Reports() <-chan *CPUWaitReport {
	return p.reportChan
}

// GetSamples 获取原始采样
func (p *CPUWaitProfiler) GetSamples() []*CPUWaitSample {
	p.samplesMu.RLock()
	defer p.samplesMu.RUnlock()

	samples := make([]*CPUWaitSample, len(p.samples))
	copy(samples, p.samples)
	return samples
}

// ClearSamples 清除采样数据
func (p *CPUWaitProfiler) ClearSamples() {
	p.samplesMu.Lock()
	defer p.samplesMu.Unlock()

	p.samples = p.samples[:0]
}

// SetSampleRate 设置采样率
func (p *CPUWaitProfiler) SetSampleRate(rate int) {
	p.sampleRate.Store(int32(rate))
	p.updateBPFSampleRate(int32(rate))
}

// GetSampleRate 获取当前采样率
func (p *CPUWaitProfiler) GetSampleRate() int {
	return int(p.sampleRate.Load())
}

// ============================================================
// 辅助函数
// ============================================================

func (p *CPUWaitProfiler) isLanguageEnabled(lang string) bool {
	for _, l := range p.config.TargetLanguages {
		if l == lang {
			return true
		}
	}
	return false
}

func (p *CPUWaitProfiler) isTargetPID(pid uint32) bool {
	p.pidMu.RLock()
	defer p.pidMu.RUnlock()

	if len(p.targetPIDs) == 0 {
		return true // 空列表表示监控所有进程
	}

	return p.targetPIDs[pid]
}

func (p *CPUWaitProfiler) detectLanguage(pid uint32) LanguageType {
	// 读取/proc/<pid>/exe
	exePath := fmt.Sprintf("/proc/%d/exe", pid)
	
	// 检查Go程序
	if p.isGoBinary(exePath) {
		return LangGo
	}

	// 检查Java程序
	comm := p.readComm(pid)
	if strings.Contains(comm, "java") {
		return LangJava
	}

	// 默认C/C++
	return LangC
}

func (p *CPUWaitProfiler) isGoBinary(path string) bool {
	// 读取ELF文件检查Go build info
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// 检查ELF magic
	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return false
	}

	if string(magic) != "\x7fELF" {
		return false
	}

	// 搜索Go build ID
	// 简化检查：查找"go.buildid"字符串
	data := make([]byte, 4096)
	f.Read(data)
	return strings.Contains(string(data), "go.buildid") || 
	       strings.Contains(string(data), "runtime.main")
}

func (p *CPUWaitProfiler) readComm(pid uint32) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (p *CPUWaitProfiler) findJavaProcesses() []uint32 {
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

		comm := p.readComm(uint32(pid))
		if strings.Contains(comm, "java") {
			pids = append(pids, uint32(pid))
		}
	}

	return pids
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

func bytesTrimNull(b []byte) []byte {
	for i, c := range b {
		if c == 0 {
			return b[:i]
		}
	}
	return b
}

// ============================================================
// 符号解析器实现（占位符）
// ============================================================

// NativeSymbolizer C/C++符号解析器
type NativeSymbolizer struct {
	symbols map[uint32]map[uint64]*StackFrame
	mu      sync.RWMutex
}

func NewNativeSymbolizer() *NativeSymbolizer {
	return &NativeSymbolizer{
		symbols: make(map[uint32]map[uint64]*StackFrame),
	}
}

func (s *NativeSymbolizer) LoadSymbols(pid uint32) error {
	// 读取/proc/<pid>/maps和/proc/<pid>/exe的符号表
	// 实际实现需要使用debug/elf解析
	return nil
}

func (s *NativeSymbolizer) UnloadSymbols(pid uint32) {
	s.mu.Lock()
	delete(s.symbols, pid)
	s.mu.Unlock()
}

func (s *NativeSymbolizer) Resolve(addr uint64, pid uint32) (*StackFrame, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pidSymbols, ok := s.symbols[pid]; ok {
		if frame, ok := pidSymbols[addr]; ok {
			return frame, nil
		}
	}

	return &StackFrame{
		Address: addr,
		Symbol:  fmt.Sprintf("0x%x", addr),
	}, nil
}

// GoSymbolizer Go符号解析器
type GoSymbolizer struct {
	NativeSymbolizer
}

func NewGoSymbolizer() *GoSymbolizer {
	return &GoSymbolizer{
		NativeSymbolizer: *NewNativeSymbolizer(),
	}
}

func (s *GoSymbolizer) LoadSymbols(pid uint32) error {
	// Go程序需要特殊处理：
	// 1. 读取ELF的.gosymtab和.gopclntab
	// 2. 解析Go特定的符号表格式
	return s.NativeSymbolizer.LoadSymbols(pid)
}

// JavaSymbolizer Java符号解析器
type JavaSymbolizer struct {
	// Java需要JVMTI或AsyncGetCallTrace
	// 这里提供框架实现
}

func NewJavaSymbolizer() *JavaSymbolizer {
	return &JavaSymbolizer{}
}

func (s *JavaSymbolizer) LoadSymbols(pid uint32) error {
	// Java符号解析需要：
	// 1. 使用perf-map-agent生成符号映射
	// 2. 或者使用JVMTI attach
	return nil
}

func (s *JavaSymbolizer) UnloadSymbols(pid uint32) {}

func (s *JavaSymbolizer) Resolve(addr uint64, pid uint32) (*StackFrame, error) {
	// 首先尝试从perf-<pid>.map文件读取
	frame, err := s.resolveFromPerfMap(addr, pid)
	if err == nil {
		return frame, nil
	}

	return &StackFrame{
		Address: addr,
		Symbol:  fmt.Sprintf("0x%x", addr),
	}, nil
}

func (s *JavaSymbolizer) resolveFromPerfMap(addr uint64, pid uint32) (*StackFrame, error) {
	mapFile := fmt.Sprintf("/tmp/perf-%d.map", pid)
	data, err := os.ReadFile(mapFile)
	if err != nil {
		return nil, err
	}

	// 解析perf map格式: <start> <size> <symbol>
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		start, err := strconv.ParseUint(fields[0], 16, 64)
		if err != nil {
			continue
		}

		size, err := strconv.ParseUint(fields[1], 16, 64)
		if err != nil {
			continue
		}

		if addr >= start && addr < start+size {
			return &StackFrame{
				Address: addr,
				Symbol:  fields[2],
				Offset:  addr - start,
			}, nil
		}
	}

	return nil, fmt.Errorf("symbol not found")
}
