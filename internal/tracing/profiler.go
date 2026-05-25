// Package tracing 进程剖析器
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package tracing

import (
	"context"
	"fmt"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

// ProcessProfiler 进程剖析器
type ProcessProfiler struct {
	sampleRate float64
	profiles   map[string]*ProcessProfile
	mu         sync.RWMutex
}

// NewProcessProfiler 创建进程剖析器
func NewProcessProfiler(sampleRate float64) *ProcessProfiler {
	return &ProcessProfiler{
		sampleRate: sampleRate,
		profiles:   make(map[string]*ProcessProfile),
	}
}

// StartProfiling 开始剖析
func (p *ProcessProfiler) StartProfiling() *ProcessProfile {
	profile := &ProcessProfile{
		StartTime: time.Now(),
	}

	// 获取当前内存状态
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	profile.MemoryInUse = m.Alloc

	// 获取当前goroutine数量
	profile.Goroutines = runtime.NumGoroutine()

	// 获取当前PID和TID
	profile.PID = uint32(getPID())
	profile.TID = uint32(getTID())

	return profile
}

// EndProfiling 结束剖析
func (p *ProcessProfiler) EndProfiling(profile *ProcessProfile) {
	if profile == nil {
		return
	}

	profile.EndTime = time.Now()
	profile.Duration = profile.EndTime.Sub(profile.StartTime)

	// 获取CPU时间（简化实现）
	profile.CPUTime = profile.Duration // 实际应该使用更精确的测量

	// 获取内存变化
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	profile.MemoryAlloc = m.TotalAlloc - profile.MemoryInUse
	profile.MemoryInUse = m.Alloc

	// 获取goroutine数量变化
	profile.Goroutines = runtime.NumGoroutine()

	// 获取系统调用次数（简化实现）
	profile.Syscalls = 0 // 实际应该使用更精确的测量

	// 获取IO统计（简化实现）
	profile.IOReadBytes = 0  // 实际应该使用更精确的测量
	profile.IOWriteBytes = 0 // 实际应该使用更精确的测量

	// 获取栈跟踪（简化实现）
	buf := make([]byte, 1024*1024)
	n := runtime.Stack(buf, false)
	profile.StackTrace = []string{string(buf[:n])}
}

// ProfileWithContext 带上下文的剖析
func (p *ProcessProfiler) ProfileWithContext(ctx context.Context, operation string, fn func()) *ProcessProfile {
	profile := p.StartProfiling()
	defer p.EndProfiling(profile)

	fn()

	return profile
}

// GetProfile 获取剖析数据
func (p *ProcessProfiler) GetProfile(key string) *ProcessProfile {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.profiles[key]
}

// SaveProfile 保存剖析数据
func (p *ProcessProfiler) SaveProfile(key string, profile *ProcessProfile) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.profiles[key] = profile
}

// DeleteProfile 删除剖析数据
func (p *ProcessProfiler) DeleteProfile(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.profiles, key)
}

// GetAllProfiles 获取所有剖析数据
func (p *ProcessProfiler) GetAllProfiles() map[string]*ProcessProfile {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*ProcessProfile)
	for k, v := range p.profiles {
		result[k] = v
	}
	return result
}

// ClearProfiles 清除所有剖析数据
func (p *ProcessProfiler) ClearProfiles() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.profiles = make(map[string]*ProcessProfile)
}

// CPUProfile CPU剖析
func (p *ProcessProfiler) CPUProfile(duration time.Duration) ([]byte, error) {
	buf := make([]byte, 0, 1024*1024)

	// 开始CPU剖析
	if err := pprof.StartCPUProfile(&buf); err != nil {
		return nil, fmt.Errorf("failed to start CPU profile: %w", err)
	}

	// 等待指定时间
	time.Sleep(duration)

	// 停止CPU剖析
	pprof.StopCPUProfile()

	return buf, nil
}

// MemoryProfile 内存剖析
func (p *ProcessProfiler) MemoryProfile() ([]byte, error) {
	buf := make([]byte, 0, 1024*1024)

	// 获取内存剖析
	if err := pprof.WriteHeapProfile(&buf); err != nil {
		return nil, fmt.Errorf("failed to write heap profile: %w", err)
	}

	return buf, nil
}

// GoroutineProfile Goroutine剖析
func (p *ProcessProfiler) GoroutineProfile() ([]byte, error) {
	buf := make([]byte, 0, 1024*1024)

	// 获取goroutine剖析
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("failed to write goroutine profile: %w", err)
	}

	return buf, nil
}

// BlockProfile 阻塞剖析
func (p *ProcessProfiler) BlockProfile() ([]byte, error) {
	buf := make([]byte, 0, 1024*1024)

	// 获取阻塞剖析
	if err := pprof.Lookup("block").WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("failed to write block profile: %w", err)
	}

	return buf, nil
}

// MutexProfile 互斥锁剖析
func (p *ProcessProfiler) MutexProfile() ([]byte, error) {
	buf := make([]byte, 0, 1024*1024)

	// 获取互斥锁剖析
	if err := pprof.Lookup("mutex").WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("failed to write mutex profile: %w", err)
	}

	return buf, nil
}

// ThreadCreateProfile 线程创建剖析
func (p *ProcessProfiler) ThreadCreateProfile() ([]byte, error) {
	buf := make([]byte, 0, 1024*1024)

	// 获取线程创建剖析
	if err := pprof.Lookup("threadcreate").WriteTo(&buf, 0); err != nil {
		return nil, fmt.Errorf("failed to write threadcreate profile: %w", err)
	}

	return buf, nil
}

// TraceProfile 执行追踪
func (p *ProcessProfiler) TraceProfile(duration time.Duration) ([]byte, error) {
	buf := make([]byte, 0, 1024*1024)

	// 开始执行追踪
	if err := runtime.StartTrace(); err != nil {
		return nil, fmt.Errorf("failed to start trace: %w", err)
	}

	// 等待指定时间
	time.Sleep(duration)

	// 停止执行追踪
	runtime.StopTrace()

	return buf, nil
}

// AllProfileTypes 所有剖析类型
var AllProfileTypes = []string{
	"cpu",
	"heap",
	"goroutine",
	"block",
	"mutex",
	"threadcreate",
}

// GetProfileByType 按类型获取剖析
func (p *ProcessProfiler) GetProfileByType(profileType string) ([]byte, error) {
	switch profileType {
	case "cpu":
		return p.CPUProfile(30 * time.Second)
	case "heap":
		return p.MemoryProfile()
	case "goroutine":
		return p.GoroutineProfile()
	case "block":
		return p.BlockProfile()
	case "mutex":
		return p.MutexProfile()
	case "threadcreate":
		return p.ThreadCreateProfile()
	default:
		return nil, fmt.Errorf("unknown profile type: %s", profileType)
	}
}
