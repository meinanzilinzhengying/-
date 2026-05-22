/*
 * Cloud Flow Agent - CPU Profiler (ON-CPU/OFF-CPU)
 *
 * 核心性能剖析实现，支持 ON-CPU 和 OFF-CPU 分析
 * 用于定位进程 CPU 瓶颈和阻塞等待问题
 */

package profiler

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ProfileType 剖析类型
type ProfileType string

const (
	ProfileTypeOnCPU   ProfileType = "on_cpu"   // CPU 执行时间
	ProfileTypeOffCPU  ProfileType = "off_cpu"  // 阻塞等待时间
	ProfileTypeMemory  ProfileType = "memory"   // 内存分配
	ProfileTypeMutex   ProfileType = "mutex"    // 锁竞争
	ProfileTypeGoroutine ProfileType = "goroutine" // Goroutine
)

// ProfilerConfig 剖析器配置
type ProfilerConfig struct {
	Enabled           bool          // 启用剖析
	OnCPUEnabled      bool          // ON-CPU 剖析
	OffCPUEnabled     bool          // OFF-CPU 剖析
	SampleRate        int           // 采样频率 (Hz)
	Duration          time.Duration // 单次剖析时长
	Interval          time.Duration // 剖析间隔
	MaxProfiles       int           // 最大保留剖析文件数
	OutputDir         string        // 输出目录
	SymbolizeEnabled  bool          // 符号化
	MinSampleCount    int           // 最小样本数
	TopN              int           // Top N 热点
}

// DefaultProfilerConfig 默认配置
func DefaultProfilerConfig() *ProfilerConfig {
	return &ProfilerConfig{
		Enabled:          true,
		OnCPUEnabled:     true,
		OffCPUEnabled:    true,
		SampleRate:       99, // 99Hz 避免与某些定时器对齐
		Duration:         30 * time.Second,
		Interval:         5 * time.Minute,
		MaxProfiles:      100,
		OutputDir:        "/var/lib/cloud-flow/profiles",
		SymbolizeEnabled: true,
		MinSampleCount:   10,
		TopN:             20,
	}
}

// ProfileResult 剖析结果
type ProfileResult struct {
	Type        ProfileType       `json:"type"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time"`
	Duration    time.Duration     `json:"duration"`
	SampleCount int               `json:"sample_count"`
	Samples     []*Sample         `json:"samples"`
	Hotspots    []*Hotspot        `json:"hotspots"`
	FlameGraph  *FlameGraphNode   `json:"flame_graph,omitempty"`
	RawData     []byte            `json:"-"`
	FilePath    string            `json:"file_path"`
}

// Sample 样本
type Sample struct {
	Stack     []string  `json:"stack"`
	Count     int       `json:"count"`
	Value     int64     `json:"value"` // CPU 周期数或等待时间
	Timestamp time.Time `json:"timestamp"`
}

// Hotspot 热点
type Hotspot struct {
	Function  string  `json:"function"`
	File      string  `json:"file"`
	Line      int     `json:"line"`
	Count     int     `json:"count"`
	Percent   float64 `json:"percent"`
	CumCount  int     `json:"cum_count"`
	CumPercent float64 `json:"cum_percent"`
}

// FlameGraphNode 火焰图节点
type FlameGraphNode struct {
	Name     string           `json:"name"`
	Value    int              `json:"value"`
	Children []*FlameGraphNode `json:"children"`
}

// CPUProfiler CPU 剖析器
type CPUProfiler struct {
	mu sync.RWMutex

	config *ProfilerConfig

	// 运行状态
	running    bool
	stopCh     chan struct{}

	// 结果存储
	results    []*ProfileResult
	resultMu   sync.RWMutex

	// 统计
	stats      ProfilerStats

	// eBPF 相关
	ebpfEnabled bool
	ebpfLoader  *EBPFLoader
}

// ProfilerStats 剖析统计
type ProfilerStats struct {
	TotalProfiles   uint64
	OnCPUProfiles   uint64
	OffCPUProfiles  uint64
	TotalSamples    uint64
	FailedProfiles  uint64
}

// EBPFLoader eBPF 加载器接口
type EBPFLoader struct {
	// eBPF 程序加载和管理
}

// NewCPUProfiler 创建 CPU 剖析器
func NewCPUProfiler(config *ProfilerConfig) (*CPUProfiler, error) {
	if config == nil {
		config = DefaultProfilerConfig()
	}

	// 确保输出目录存在
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create profile directory: %w", err)
	}

	return &CPUProfiler{
		config:  config,
		stopCh:  make(chan struct{}),
		results: make([]*ProfileResult, 0),
	}, nil
}

// Start 启动剖析器
func (p *CPUProfiler) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	p.running = true

	// 启动定时剖析
	go p.profileLoop()

	return nil
}

// Stop 停止剖析器
func (p *CPUProfiler) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.running = false
	close(p.stopCh)

	return nil
}

// profileLoop 剖析循环
func (p *CPUProfiler) profileLoop() {
	ticker := time.NewTicker(p.config.Interval)
	defer ticker.Stop()

	// 立即执行一次
	p.runProfile()

	for {
		select {
		case <-ticker.C:
			p.runProfile()
		case <-p.stopCh:
			return
		}
	}
}

// runProfile 执行剖析
func (p *CPUProfiler) runProfile() {
	if p.config.OnCPUEnabled {
		result, err := p.ProfileOnCPU()
		if err != nil {
			atomic.AddUint64(&p.stats.FailedProfiles, 1)
		} else {
			p.saveResult(result)
			atomic.AddUint64(&p.stats.OnCPUProfiles, 1)
		}
	}

	if p.config.OffCPUEnabled {
		result, err := p.ProfileOffCPU()
		if err != nil {
			atomic.AddUint64(&p.stats.FailedProfiles, 1)
		} else {
			p.saveResult(result)
			atomic.AddUint64(&p.stats.OffCPUProfiles, 1)
		}
	}

	atomic.AddUint64(&p.stats.TotalProfiles, 1)
}

// ProfileOnCPU ON-CPU 剖析
func (p *CPUProfiler) ProfileOnCPU() (*ProfileResult, error) {
	startTime := time.Now()

	// 创建临时文件
	tmpFile, err := os.CreateTemp(p.config.OutputDir, "oncpu_*.prof")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// 启动 CPU 剖析
	if err := pprof.StartCPUProfile(tmpFile); err != nil {
		return nil, fmt.Errorf("failed to start CPU profile: %w", err)
	}

	// 采样指定时长
	time.Sleep(p.config.Duration)
	pprof.StopCPUProfile()

	// 读取剖析数据
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read profile data: %w", err)
	}

	// 解析剖析数据
	samples, err := p.parseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}

	// 生成火焰图
	flameGraph := p.buildFlameGraph(samples)

	// 计算热点
	hotspots := p.calculateHotspots(samples)

	result := &ProfileResult{
		Type:        ProfileTypeOnCPU,
		StartTime:   startTime,
		EndTime:     time.Now(),
		Duration:    p.config.Duration,
		SampleCount: len(samples),
		Samples:     samples,
		Hotspots:    hotspots,
		FlameGraph:  flameGraph,
		RawData:     data,
		FilePath:    tmpFile.Name(),
	}

	atomic.AddUint64(&p.stats.TotalSamples, uint64(len(samples)))

	return result, nil
}

// ProfileOffCPU OFF-CPU 剖析
func (p *CPUProfiler) ProfileOffCPU() (*ProfileResult, error) {
	startTime := time.Now()

	// OFF-CPU 剖析需要 eBPF 支持
	// 这里使用简化实现，实际应使用 eBPF 程序
	if p.ebpfEnabled && p.ebpfLoader != nil {
		return p.profileOffCPUEBPF()
	}

	// 使用 runtime 的阻塞剖析作为替代
	tmpFile, err := os.CreateTemp(p.config.OutputDir, "offcpu_*.prof")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// 记录阻塞事件
	blockProfile := pprof.Lookup("block")
	if blockProfile != nil {
		if err := blockProfile.WriteTo(tmpFile, 0); err != nil {
			return nil, fmt.Errorf("failed to write block profile: %w", err)
		}
	}

	// 读取数据
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read profile data: %w", err)
	}

	samples, err := p.parseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}

	flameGraph := p.buildFlameGraph(samples)
	hotspots := p.calculateHotspots(samples)

	result := &ProfileResult{
		Type:        ProfileTypeOffCPU,
		StartTime:   startTime,
		EndTime:     time.Now(),
		Duration:    p.config.Duration,
		SampleCount: len(samples),
		Samples:     samples,
		Hotspots:    hotspots,
		FlameGraph:  flameGraph,
		RawData:     data,
		FilePath:    tmpFile.Name(),
	}

	return result, nil
}

// profileOffCPUEBPF 使用 eBPF 进行 OFF-CPU 剖析
func (p *CPUProfiler) profileOffCPUEBPF() (*ProfileResult, error) {
	// 实际实现应加载 eBPF 程序跟踪 sched_switch 事件
	// 计算进程离开 CPU 的时间
	return nil, fmt.Errorf("eBPF OFF-CPU profiling not implemented")
}

// parseProfile 解析剖析数据
func (p *CPUProfiler) parseProfile(data []byte) ([]*Sample, error) {
	samples := make([]*Sample, 0)

	// 解析 pprof 格式
	reader := bufio.NewReader(bytes.NewReader(data))
	var currentSample *Sample

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)

		// 解析样本计数
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// 简化的解析逻辑
		// 实际应使用 github.com/google/pprof 库解析
		if currentSample == nil {
			currentSample = &Sample{
				Stack:     make([]string, 0),
				Timestamp: time.Now(),
			}
		}

		// 解析调用栈
		if strings.Contains(line, ":") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				function := strings.TrimSpace(parts[0])
				currentSample.Stack = append(currentSample.Stack, function)
			}
		}

		// 样本结束标记
		if strings.HasPrefix(line, "---") && currentSample != nil {
			if len(currentSample.Stack) > 0 {
				samples = append(samples, currentSample)
			}
			currentSample = nil
		}
	}

	return samples, nil
}

// buildFlameGraph 构建火焰图
func (p *CPUProfiler) buildFlameGraph(samples []*Sample) *FlameGraphNode {
	root := &FlameGraphNode{
		Name:     "root",
		Value:    0,
		Children: make([]*FlameGraphNode, 0),
	}

	// 按调用栈聚合
	stackMap := make(map[string]int)
	for _, sample := range samples {
		// 反转调用栈（从根到叶）
		stackKey := ""
		for i := len(sample.Stack) - 1; i >= 0; i-- {
			if stackKey != "" {
				stackKey += ";"
			}
			stackKey += sample.Stack[i]
		}
		stackMap[stackKey] += sample.Count
	}

	// 构建树
	for stackKey, count := range stackMap {
		frames := strings.Split(stackKey, ";")
		p.addToFlameGraph(root, frames, count)
	}

	return root
}

// addToFlameGraph 添加到火焰图
func (p *CPUProfiler) addToFlameGraph(node *FlameGraphNode, frames []string, value int) {
	if len(frames) == 0 {
		node.Value += value
		return
	}

	frame := frames[0]
	var child *FlameGraphNode

	for _, c := range node.Children {
		if c.Name == frame {
			child = c
			break
		}
	}

	if child == nil {
		child = &FlameGraphNode{
			Name:     frame,
			Value:    0,
			Children: make([]*FlameGraphNode, 0),
		}
		node.Children = append(node.Children, child)
	}

	p.addToFlameGraph(child, frames[1:], value)
}

// calculateHotspots 计算热点
func (p *CPUProfiler) calculateHotspots(samples []*Sample) []*Hotspot {
	// 统计函数出现次数
	funcCount := make(map[string]int)
	funcCumCount := make(map[string]int)

	for _, sample := range samples {
		seen := make(map[string]bool)
		for i, frame := range sample.Stack {
			funcCumCount[frame] += sample.Count
			if !seen[frame] {
				funcCount[frame] += sample.Count
				seen[frame] = true
			}
			// 叶节点额外计数
			if i == 0 {
				funcCount[frame] += sample.Count
			}
		}
	}

	// 转换为热点列表
	hotspots := make([]*Hotspot, 0, len(funcCount))
	totalCount := 0
	for _, count := range funcCount {
		totalCount += count
	}

	for funcName, count := range funcCount {
		percent := float64(count) * 100 / float64(totalCount)
		cumPercent := float64(funcCumCount[funcName]) * 100 / float64(totalCount)

		hotspots = append(hotspots, &Hotspot{
			Function:   funcName,
			Count:      count,
			Percent:    percent,
			CumCount:   funcCumCount[funcName],
			CumPercent: cumPercent,
		})
	}

	// 按计数排序
	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].Count > hotspots[j].Count
	})

	// 取 Top N
	if len(hotspots) > p.config.TopN {
		hotspots = hotspots[:p.config.TopN]
	}

	return hotspots
}

// saveResult 保存结果
func (p *CPUProfiler) saveResult(result *ProfileResult) {
	p.resultMu.Lock()
	defer p.resultMu.Unlock()

	p.results = append(p.results, result)

	// 清理旧结果
	if len(p.results) > p.config.MaxProfiles {
		// 删除旧文件
		oldResult := p.results[0]
		if oldResult.FilePath != "" {
			os.Remove(oldResult.FilePath)
		}
		p.results = p.results[1:]
	}
}

// GetLatestResult 获取最新结果
func (p *CPUProfiler) GetLatestResult(profileType ProfileType) *ProfileResult {
	p.resultMu.RLock()
	defer p.resultMu.RUnlock()

	for i := len(p.results) - 1; i >= 0; i-- {
		if p.results[i].Type == profileType {
			return p.results[i]
		}
	}

	return nil
}

// GetAllResults 获取所有结果
func (p *CPUProfiler) GetAllResults() []*ProfileResult {
	p.resultMu.RLock()
	defer p.resultMu.RUnlock()

	results := make([]*ProfileResult, len(p.results))
	copy(results, p.results)
	return results
}

// GetStats 获取统计
func (p *CPUProfiler) GetStats() ProfilerStats {
	return ProfilerStats{
		TotalProfiles:  atomic.LoadUint64(&p.stats.TotalProfiles),
		OnCPUProfiles:  atomic.LoadUint64(&p.stats.OnCPUProfiles),
		OffCPUProfiles: atomic.LoadUint64(&p.stats.OffCPUProfiles),
		TotalSamples:   atomic.LoadUint64(&p.stats.TotalSamples),
		FailedProfiles: atomic.LoadUint64(&p.stats.FailedProfiles),
	}
}

// ProfileNow 立即执行剖析
func (p *CPUProfiler) ProfileNow(profileType ProfileType) (*ProfileResult, error) {
	switch profileType {
	case ProfileTypeOnCPU:
		return p.ProfileOnCPU()
	case ProfileTypeOffCPU:
		return p.ProfileOffCPU()
	default:
		return nil, fmt.Errorf("unsupported profile type: %s", profileType)
	}
}
