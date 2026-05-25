// Package isolation 业务安全隔离
// 部署/启停/卸载进程隔离，cgroup限制，零业务干扰
package isolation

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ============================================================
// 隔离配置
// ============================================================

// IsolationConfig 隔离配置
type IsolationConfig struct {
	Enabled           bool   `yaml:"enabled" json:"enabled"`
	CgroupPath        string `yaml:"cgroup_path" json:"cgroup_path"`               // cgroup挂载路径
	CPUQuotaPercent   int    `yaml:"cpu_quota_percent" json:"cpu_quota_percent"`   // CPU限制百分比
	MemoryLimitMB     int    `yaml:"memory_limit_mb" json:"memory_limit_mb"`       // 内存限制MB
	IOWeight          int    `yaml:"io_weight" json:"io_weight"`                   // IO权重 10-1000
	NiceLevel         int    `yaml:"nice_level" json:"nice_level"`                 // nice值 19(最低)-(-20)
	OOMPriority       int    `yaml:"oom_priority" json:"oom_priority"`             // OOM优先级 1000(最先杀)-(-1000)
	NoNewPrivileges   bool   `yaml:"no_new_privileges" json:"no_new_privileges"`   // 禁止提权
	SeccompEnabled    bool   `yaml:"seccomp_enabled" json:"seccomp_enabled"`       // 启用seccomp
	DropCapabilities  bool   `yaml:"drop_capabilities" json:"drop_capabilities"`   // 丢弃特权
}

// DefaultConfig 默认配置
func DefaultConfig() *IsolationConfig {
	return &IsolationConfig{
		Enabled:         true,
		CgroupPath:      "/sys/fs/cgroup",
		CPUQuotaPercent: 10,     // 限制10% CPU
		MemoryLimitMB:   512,    // 限制512MB内存
		IOWeight:        10,     // 最低IO权重
		NiceLevel:       19,     // 最低调度优先级
		OOMPriority:     1000,   // OOM时优先被杀
		NoNewPrivileges: true,
		SeccompEnabled:  true,
		DropCapabilities: true,
	}
}

// ============================================================
// 隔离管理器
// ============================================================

// Manager 隔离管理器
type Manager struct {
	config      *IsolationConfig
	cgroupMgr   *CgroupManager
	processMgr  *ProcessManager
}

// NewManager 创建隔离管理器
func NewManager(config *IsolationConfig) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	cgroupMgr, err := NewCgroupManager(config)
	if err != nil {
		return nil, fmt.Errorf("创建cgroup管理器失败: %w", err)
	}

	return &Manager{
		config:     config,
		cgroupMgr:  cgroupMgr,
		processMgr: NewProcessManager(config),
	}, nil
}

// ApplyIsolation 应用隔离（在子进程中调用）
func (m *Manager) ApplyIsolation() error {
	if !m.config.Enabled {
		return nil
	}

	// 1. 设置进程优先级
	if err := m.processMgr.SetPriority(); err != nil {
		return fmt.Errorf("设置进程优先级失败: %w", err)
	}

	// 2. 设置OOM分数
	if err := m.processMgr.SetOOMPriority(); err != nil {
		return fmt.Errorf("设置OOM优先级失败: %w", err)
	}

	// 3. 加入cgroup
	if err := m.cgroupMgr.Join(); err != nil {
		return fmt.Errorf("加入cgroup失败: %w", err)
	}

	// 4. 限制权限
	if err := m.processMgr.DropPrivileges(); err != nil {
		return fmt.Errorf("限制权限失败: %w", err)
	}

	return nil
}

// Cleanup 清理隔离资源
func (m *Manager) Cleanup() error {
	if !m.config.Enabled {
		return nil
	}
	return m.cgroupMgr.Remove()
}

// ============================================================
// cgroup管理器
// ============================================================

// CgroupManager cgroup管理器
type CgroupManager struct {
	config    *IsolationConfig
	groupName string
	paths     map[string]string // subsystem -> path
}

// NewCgroupManager 创建cgroup管理器
func NewCgroupManager(config *IsolationConfig) (*CgroupManager, error) {
	mgr := &CgroupManager{
		config:    config,
		groupName: "cloud-flow-agent",
		paths:     make(map[string]string),
	}

	// 检测cgroup版本和可用subsystem
	if err := mgr.detectCgroup(); err != nil {
		return nil, err
	}

	return mgr, nil
}

// detectCgroup 检测cgroup配置
func (c *CgroupManager) detectCgroup() error {
	// 检查cgroup v2
	if _, err := os.Stat(filepath.Join(c.config.CgroupPath, "cgroup.controllers")); err == nil {
		// cgroup v2
		c.paths["v2"] = filepath.Join(c.config.CgroupPath, c.groupName)
		return nil
	}

	// cgroup v1 - 检查各subsystem
	subsystems := []string{"cpu", "cpuacct", "memory", "blkio", "pids"}
	for _, sub := range subsystems {
		path := filepath.Join(c.config.CgroupPath, sub, c.groupName)
		if _, err := os.Stat(filepath.Join(c.config.CgroupPath, sub)); err == nil {
			c.paths[sub] = path
		}
	}

	return nil
}

// Create 创建cgroup
func (c *CgroupManager) Create() error {
	// cgroup v2
	if v2Path, ok := c.paths["v2"]; ok {
		if err := os.MkdirAll(v2Path, 0755); err != nil {
			return err
		}

		// 设置CPU限制 (10%)
		cpuMax := fmt.Sprintf("%d %d", c.config.CPUQuotaPercent*1000, 100000)
		if err := os.WriteFile(filepath.Join(v2Path, "cpu.max"), []byte(cpuMax), 0644); err != nil {
			return fmt.Errorf("设置CPU限制失败: %w", err)
		}

		// 设置内存限制
		memLimit := fmt.Sprintf("%d", c.config.MemoryLimitMB*1024*1024)
		if err := os.WriteFile(filepath.Join(v2Path, "memory.max"), []byte(memLimit), 0644); err != nil {
			return fmt.Errorf("设置内存限制失败: %w", err)
		}

		// 设置IO权重
		ioWeight := fmt.Sprintf("%d", c.config.IOWeight)
		if err := os.WriteFile(filepath.Join(v2Path, "io.weight"), []byte(ioWeight), 0644); err != nil {
			return fmt.Errorf("设置IO权重失败: %w", err)
		}

		return nil
	}

	// cgroup v1
	// CPU限制
	if cpuPath, ok := c.paths["cpu"]; ok {
		if err := os.MkdirAll(cpuPath, 0755); err != nil {
			return err
		}
		// cfs_quota_us = cpu.cfs_period_us * CPU百分比 / 100
		period := 100000 // 100ms
		quota := period * c.config.CPUQuotaPercent / 100
		os.WriteFile(filepath.Join(cpuPath, "cpu.cfs_period_us"), []byte(strconv.Itoa(period)), 0644)
		os.WriteFile(filepath.Join(cpuPath, "cpu.cfs_quota_us"), []byte(strconv.Itoa(quota)), 0644)
	}

	// 内存限制
	if memPath, ok := c.paths["memory"]; ok {
		if err := os.MkdirAll(memPath, 0755); err != nil {
			return err
		}
		limit := fmt.Sprintf("%d", c.config.MemoryLimitMB*1024*1024)
		os.WriteFile(filepath.Join(memPath, "memory.limit_in_bytes"), []byte(limit), 0644)
	}

	// IO限制
	if blkioPath, ok := c.paths["blkio"]; ok {
		if err := os.MkdirAll(blkioPath, 0755); err != nil {
			return err
		}
		weight := fmt.Sprintf("%d", c.config.IOWeight)
		os.WriteFile(filepath.Join(blkioPath, "blkio.weight"), []byte(weight), 0644)
	}

	return nil
}

// Join 将当前进程加入cgroup
func (c *CgroupManager) Join() error {
	pid := strconv.Itoa(os.Getpid())

	// cgroup v2
	if v2Path, ok := c.paths["v2"]; ok {
		procsFile := filepath.Join(v2Path, "cgroup.procs")
		return os.WriteFile(procsFile, []byte(pid), 0644)
	}

	// cgroup v1
	for _, path := range c.paths {
		procsFile := filepath.Join(path, "cgroup.procs")
		os.WriteFile(procsFile, []byte(pid), 0644)
	}

	return nil
}

// Remove 删除cgroup
func (c *CgroupManager) Remove() error {
	// cgroup v2
	if v2Path, ok := c.paths["v2"]; ok {
		return os.RemoveAll(v2Path)
	}

	// cgroup v1
	for _, path := range c.paths {
		os.RemoveAll(path)
	}

	return nil
}

// ============================================================
// 进程管理器
// ============================================================

// ProcessManager 进程管理器
type ProcessManager struct {
	config *IsolationConfig
}

// NewProcessManager 创建进程管理器
func NewProcessManager(config *IsolationConfig) *ProcessManager {
	return &ProcessManager{config: config}
}

// SetPriority 设置进程优先级
func (p *ProcessManager) SetPriority() error {
	// 设置nice值 (19 = 最低优先级)
	return syscall.Setpriority(syscall.PRIO_PROCESS, 0, p.config.NiceLevel)
}

// SetOOMPriority 设置OOM优先级
func (p *ProcessManager) SetOOMPriority() error {
	// /proc/[pid]/oom_score_adj
	// 1000 = OOM时最先被杀
	scoreAdj := fmt.Sprintf("%d", p.config.OOMPriority)
	return os.WriteFile(fmt.Sprintf("/proc/self/oom_score_adj"), []byte(scoreAdj), 0644)
}

// DropPrivileges 丢弃特权
func (p *ProcessManager) DropPrivileges() error {
	if !p.config.DropCapabilities {
		return nil
	}

	// 设置no_new_privs
	if p.config.NoNewPrivileges {
		// prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
		_, _, err := syscall.Syscall6(syscall.SYS_PRCTL, 38, 1, 0, 0, 0, 0)
		if err != 0 {
			return fmt.Errorf("设置no_new_privs失败: %v", err)
		}
	}

	// 注意：完整的capabilities管理需要libcap或execve调用capsh
	// 这里简化处理，实际生产环境应使用更完善的方案

	return nil
}

// ============================================================
// 部署管理
// ============================================================

// DeployManager 部署管理器
type DeployManager struct {
	config *IsolationConfig
}

// NewDeployManager 创建部署管理器
func NewDeployManager(config *IsolationConfig) *DeployManager {
	if config == nil {
		config = DefaultConfig()
	}
	return &DeployManager{config: config}
}

// PreDeployCheck 部署前检查
func (d *DeployManager) PreDeployCheck() error {
	// 1. 检查是否有冲突进程
	if err := d.checkConflictingProcesses(); err != nil {
		return fmt.Errorf("存在冲突进程: %w", err)
	}

	// 2. 检查资源是否充足
	if err := d.checkResourceAvailability(); err != nil {
		return fmt.Errorf("资源不足: %w", err)
	}

	// 3. 检查cgroup支持
	if d.config.Enabled {
		if _, err := os.Stat(d.config.CgroupPath); err != nil {
			return fmt.Errorf("cgroup不可用: %w", err)
		}
	}

	return nil
}

// checkConflictingProcesses 检查冲突进程
func (d *DeployManager) checkConflictingProcesses() error {
	// 检查是否已有agent实例在运行
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// 读取进程名
		commPath := fmt.Sprintf("/proc/%d/comm", pid)
		data, err := os.ReadFile(commPath)
		if err != nil {
			continue
		}

		comm := strings.TrimSpace(string(data))
		if comm == "cloud-flow-agent" && pid != os.Getpid() {
			return fmt.Errorf("发现已有agent进程运行 (PID: %d)", pid)
		}
	}

	return nil
}

// checkResourceAvailability 检查资源可用性
func (d *DeployManager) checkResourceAvailability() error {
	// 检查内存
	memInfo, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return err
	}

	var memAvailable int64
	lines := strings.Split(string(memInfo), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memAvailable, _ = strconv.ParseInt(fields[1], 10, 64)
				memAvailable *= 1024 // kB to bytes
			}
			break
		}
	}

	requiredMem := int64(d.config.MemoryLimitMB) * 1024 * 1024
	if memAvailable < requiredMem {
		return fmt.Errorf("可用内存不足: %d MB < %d MB", memAvailable/1024/1024, d.config.MemoryLimitMB)
	}

	return nil
}

// SafeStart 安全启动（带隔离）
func (d *DeployManager) SafeStart(execFunc func() error) error {
	// 1. 预检查
	if err := d.PreDeployCheck(); err != nil {
		return err
	}

	// 2. 创建隔离管理器
	isolationMgr, err := NewManager(d.config)
	if err != nil {
		return fmt.Errorf("创建隔离管理器失败: %w", err)
	}
	defer isolationMgr.Cleanup()

	// 3. 创建cgroup
	if err := isolationMgr.cgroupMgr.Create(); err != nil {
		return fmt.Errorf("创建cgroup失败: %w", err)
	}

	// 4. 应用隔离并执行
	if err := isolationMgr.ApplyIsolation(); err != nil {
		return fmt.Errorf("应用隔离失败: %w", err)
	}

	return execFunc()
}

// SafeStop 安全停止
func (d *DeployManager) SafeStop() error {
	// 发送SIGTERM给agent进程
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		commPath := fmt.Sprintf("/proc/%d/comm", pid)
		data, err := os.ReadFile(commPath)
		if err != nil {
			continue
		}

		if strings.TrimSpace(string(data)) == "cloud-flow-agent" {
			// 发送SIGTERM
			syscall.Kill(pid, syscall.SIGTERM)
		}
	}

	return nil
}

// SafeUninstall 安全卸载
func (d *DeployManager) SafeUninstall() error {
	// 1. 先停止
	if err := d.SafeStop(); err != nil {
		return err
	}

	// 2. 等待进程退出
	time.Sleep(2 * time.Second)

	// 3. 清理cgroup
	isolationMgr, _ := NewManager(d.config)
	if isolationMgr != nil {
		isolationMgr.Cleanup()
	}

	// 4. 清理其他资源...

	return nil
}

// ============================================================
// 零干扰保障
// ============================================================

// ZeroInterferenceGuard 零干扰保障
type ZeroInterferenceGuard struct {
	config *IsolationConfig
}

// NewZeroInterferenceGuard 创建零干扰保障
func NewZeroInterferenceGuard(config *IsolationConfig) *ZeroInterferenceGuard {
	return &ZeroInterferenceGuard{config: config}
}

// Ensure 确保零干扰
func (z *ZeroInterferenceGuard) Ensure() error {
	if !z.config.Enabled {
		return nil
	}

	// 1. 设置最低优先级
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, 0, 19); err != nil {
		return fmt.Errorf("设置最低优先级失败: %w", err)
	}

	// 2. 设置OOM时优先被杀
	if err := os.WriteFile("/proc/self/oom_score_adj", []byte("1000"), 0644); err != nil {
		return fmt.Errorf("设置OOM优先级失败: %w", err)
	}

	// 3. 设置CPU亲和性（绑定到特定核心，避免占用业务核心）
	// 实际生产环境应根据业务部署情况动态选择

	// 4. 设置IO优先级
	// ionice -c 3 (idle class)
	// 需要调用ioprio_set系统调用

	return nil
}

// CheckInterference 检查是否对业务造成干扰
func (z *ZeroInterferenceGuard) CheckInterference() (*InterferenceReport, error) {
	report := &InterferenceReport{
		Timestamp: time.Now().Unix(),
	}

	// 1. 检查CPU使用率
	cpuUsage, err := z.getCPUUsage()
	if err == nil && cpuUsage > float64(z.config.CPUQuotaPercent)*1.5 {
		report.CPUExceeded = true
		report.Warnings = append(report.Warnings, fmt.Sprintf("CPU使用率 %.1f%% 超过配额", cpuUsage))
	}

	// 2. 检查内存使用
	memUsage, err := z.getMemoryUsage()
	if err == nil && memUsage > int64(z.config.MemoryLimitMB)*1024*1024 {
		report.MemoryExceeded = true
		report.Warnings = append(report.Warnings, "内存使用超过限制")
	}

	// 3. 检查是否影响业务进程
	if affected := z.checkBusinessImpact(); len(affected) > 0 {
		report.BusinessAffected = affected
		report.Warnings = append(report.Warnings, fmt.Sprintf("可能影响业务进程: %v", affected))
	}

	report.HasInterference = report.CPUExceeded || report.MemoryExceeded || len(report.BusinessAffected) > 0

	return report, nil
}

// getCPUUsage 获取CPU使用率
func (z *ZeroInterferenceGuard) getCPUUsage() (float64, error) {
	// 简化实现：读取/proc/stat
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	// 解析CPU行
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}

			var total, idle uint64
			for i := 1; i < len(fields); i++ {
				val, _ := strconv.ParseUint(fields[i], 10, 64)
				total += val
				if i == 4 { // idle
					idle = val
				}
			}

			if total > 0 {
				return float64(total-idle) / float64(total) * 100, nil
			}
		}
	}

	return 0, fmt.Errorf("无法解析CPU统计")
}

// getMemoryUsage 获取内存使用
func (z *ZeroInterferenceGuard) getMemoryUsage() (int64, error) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				// 单位是kB
				return val * 1024, nil
			}
		}
	}

	return 0, fmt.Errorf("无法解析内存使用")
}

// checkBusinessImpact 检查业务影响
func (z *ZeroInterferenceGuard) checkBusinessImpact() []string {
	// 简化实现：检查是否有业务进程的延迟明显增加
	// 实际生产环境应通过更精细的指标判断
	return nil
}

// InterferenceReport 干扰报告
type InterferenceReport struct {
	Timestamp        int64    `json:"timestamp"`
	HasInterference  bool     `json:"has_interference"`
	CPUExceeded      bool     `json:"cpu_exceeded"`
	MemoryExceeded   bool     `json:"memory_exceeded"`
	BusinessAffected []string `json:"business_affected,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

// ============================================================
// 工具函数
// ============================================================
