// Package collector 定义采集器核心接口
package collector

import (
	"context"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// Collector 采集器接口
type Collector interface {
	// Name 返回采集器名称
	Name() string

	// Type 返回采集器类型
	Type() models.CollectorType

	// Init 初始化采集器
	Init(ctx context.Context, config interface{}) error

	// Start 启动采集器
	Start(ctx context.Context) error

	// Stop 停止采集器
	Stop(ctx context.Context) error

	// Status 返回采集器状态
	Status() models.CollectorStatus

	// Events 返回事件通道
	Events() <-chan interface{}

	// Errors 返回错误通道
	Errors() <-chan error
}

// NetworkCollector 网络流量采集器接口
type NetworkCollector interface {
	Collector
	// Flows 返回网络流量数据通道
	Flows() <-chan *models.NetworkFlow
}

// MetricsCollector 系统指标采集器接口
type MetricsCollector interface {
	Collector
	// Metrics 返回系统指标数据通道
	Metrics() <-chan *models.SystemMetric
}

// ProcessCollector 进程事件采集器接口
type ProcessCollector interface {
	Collector
	// ProcessEvents 返回进程事件通道
	ProcessEvents() <-chan *models.ProcessEvent
}

// SyscallCollector 系统调用采集器接口
type SyscallCollector interface {
	Collector
	// Syscalls 返回系统调用事件通道
	Syscalls() <-chan *models.SyscallEvent
}

// Manager 采集器管理器接口
type Manager interface {
	// Register 注册采集器
	Register(collector Collector) error

	// Unregister 注销采集器
	Unregister(name string) error

	// Get 获取采集器
	Get(name string) (Collector, bool)

	// List 列出所有采集器
	List() []Collector

	// StartAll 启动所有采集器
	StartAll(ctx context.Context) error

	// StopAll 停止所有采集器
	StopAll(ctx context.Context) error

	// ReloadConfig 热加载配置
	ReloadConfig(ctx context.Context, config *models.CollectorsConfig) error

	// Status 返回所有采集器状态
	Status() []models.CollectorStatus
}
