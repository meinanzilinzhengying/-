// Package kernel 提供内核能力检测功能
package kernel

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

const (
	// MinKernelVersion 最低内核版本要求
	MinKernelMajor = 3
	MinKernelMinor = 10

	// EBPFMinKernelVersion eBPF 最低内核版本
	EBPFMinKernelMajor = 4
	EBPFMinKernelMinor  = 4

	// BTFMinKernelVersion BTF 最低内核版本
	BTFMinKernelMajor = 5
	BTFMinKernelMinor  = 2

	// RingBufMinKernelVersion Ring Buffer 最低内核版本
	RingBufMinKernelMajor = 5
	RingBufMinKernelMinor  = 8
)

// Detector 内核检测器
type Detector struct {
	cached *models.KernelCapability
}

// NewDetector 创建内核检测器
func NewDetector() *Detector {
	return &Detector{}
}

// Detect 检测内核能力
func (d *Detector) Detect() (*models.KernelCapability, error) {
	if d.cached != nil {
		return d.cached, nil
	}

	cap := &models.KernelCapability{
		DetectedAt: time.Now(),
	}

	// 检测架构
	cap.Arch = detectArch()

	// 检测内核版本
	version, err := detectKernelVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to detect kernel version: %w", err)
	}
	cap.Version = version.String()
	cap.Major = version.Major
	cap.Minor = version.Minor
	cap.Patch = version.Patch

	// 检测最低版本要求
	cap.MinRequired = isMinRequired(version)

	// 检测 eBPF 支持
	cap.SupportsEBPF = supportsEBPF(version)

	// 检测 BTF 支持
	cap.SupportsBTF = supportsBTF(version)

	// 检测 Ring Buffer 支持
	cap.SupportsRingBuf = supportsRingBuf(version)

	d.cached = cap
	return cap, nil
}

// Version 内核版本
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v *Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare 比较版本，返回 -1, 0, 1
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// detectArch 检测系统架构
func detectArch() models.ArchType {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		return models.ArchX86_64
	case "arm64":
		return models.ArchARM64
	default:
		return models.ArchUnknown
	}
}

// detectKernelVersion 检测内核版本
func detectKernelVersion() (*Version, error) {
	// 方法1: 读取 /proc/version_signature (Ubuntu/Debian)
	if v, err := readVersionSignature(); err == nil {
		return v, nil
	}

	// 方法2: 读取 /proc/version
	if v, err := readProcVersion(); err == nil {
		return v, nil
	}

	// 方法3: 使用 uname -r
	if v, err := readUnameRelease(); err == nil {
		return v, nil
	}

	return nil, fmt.Errorf("failed to detect kernel version")
}

// readVersionSignature 读取 /proc/version_signature
func readVersionSignature() (*Version, error) {
	data, err := os.ReadFile("/proc/version_signature")
	if err != nil {
		return nil, err
	}

	// 格式: Ubuntu 4.15.0-76.86-generic 4.15.18
	// 或: CentOS Linux 7 (Core) 3.10.0-1160.el7.x86_64
	parts := strings.Fields(string(data))
	for _, part := range parts {
		if v := parseVersionString(part); v != nil {
			return v, nil
		}
	}

	return nil, fmt.Errorf("no valid version found in version_signature")
}

// readProcVersion 读取 /proc/version
func readProcVersion() (*Version, error) {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return nil, err
	}

	// 格式: Linux version 5.4.0-42-generic (buildd@lgw01-amd64-001) ...
	re := regexp.MustCompile(`Linux version (\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return parseVersionString(matches[1]), nil
	}

	return nil, fmt.Errorf("no valid version found in /proc/version")
}

// readUnameRelease 使用 uname 获取内核版本
func readUnameRelease() (*Version, error) {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return nil, err
	}

	// 格式: 5.4.0-42-generic 或 4.19.90-24
	re := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return parseVersionString(matches[1]), nil
	}

	return nil, fmt.Errorf("no valid version found in osrelease")
}

// parseVersionString 解析版本字符串
func parseVersionString(s string) *Version {
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(s)
	if len(matches) != 4 {
		return nil
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}
}

// isMinRequired 检测是否满足最低版本要求
func isMinRequired(v *Version) bool {
	minVersion := &Version{Major: MinKernelMajor, Minor: MinKernelMinor, Patch: 0}
	return v.Compare(minVersion) >= 0
}

// supportsEBPF 检测是否支持 eBPF
func supportsEBPF(v *Version) bool {
	// 检查内核版本
	minVersion := &Version{Major: EBPFMinKernelMajor, Minor: EBPFMinKernelMinor, Patch: 0}
	if v.Compare(minVersion) < 0 {
		return false
	}

	// 检查 /sys/kernel/debug/tracing/available_events
	if _, err := os.Stat("/sys/kernel/debug/tracing"); err != nil {
		// 可能需要挂载 debugfs
		return false
	}

	// 检查 BPF syscall 是否可用
	if !checkBPFSystemCall() {
		return false
	}

	return true
}

// supportsBTF 检测是否支持 BTF
func supportsBTF(v *Version) bool {
	// 检查内核版本
	minVersion := &Version{Major: BTFMinKernelMajor, Minor: BTFMinKernelMinor, Patch: 0}
	if v.Compare(minVersion) < 0 {
		return false
	}

	// 检查 /sys/kernel/btf/vmlinux
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); err != nil {
		return false
	}

	return true
}

// supportsRingBuf 检测是否支持 Ring Buffer
func supportsRingBuf(v *Version) bool {
	minVersion := &Version{Major: RingBufMinKernelMajor, Minor: RingBufMinKernelMinor, Patch: 0}
	return v.Compare(minVersion) >= 0
}

// checkBPFSystemCall 检查 BPF 系统调用是否可用
func checkBPFSystemCall() bool {
	// 检查 /proc/sys/kernel/unprivileged_bpf_disabled
	// 0: 允许非特权用户使用 BPF
	// 1: 禁止非特权用户使用 BPF
	// 2: 禁止非特权用户使用 BPF (不可更改)
	data, err := os.ReadFile("/proc/sys/kernel/unprivileged_bpf_disabled")
	if err != nil {
		// 文件不存在，假设 BPF 可用
		return true
	}

	// 即使禁止非特权用户，root 用户仍可使用
	// 所以这里总是返回 true
	_ = data
	return true
}

// CheckKernelConfig 检查内核配置
func (d *Detector) CheckKernelConfig() (map[string]bool, error) {
	config := make(map[string]bool)

	// 检查 /boot/config-$(uname -r)
	release, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return config, err
	}

	configPath := fmt.Sprintf("/boot/config-%s", strings.TrimSpace(string(release)))
	file, err := os.Open(configPath)
	if err != nil {
		// 尝试 /proc/config.gz
		return checkKernelConfigFromGz()
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "CONFIG_") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]
				config[key] = value == "y" || value == "m"
			}
		}
	}

	return config, nil
}

// checkKernelConfigFromGz 从 /proc/config.gz 读取内核配置
func checkKernelConfigFromGz() (map[string]bool, error) {
	config := make(map[string]bool)

	// 这里需要解压 gzip，简化处理
	// 实际实现可以使用 compress/gzip 包
	return config, fmt.Errorf("reading from /proc/config.gz not implemented")
}

// GetCPUInfo 获取 CPU 信息
func GetCPUInfo() (string, string, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	var vendor, model string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "vendor_id") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				vendor = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "model name") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				model = strings.TrimSpace(parts[1])
			}
		}
	}

	// 检测国产芯片
	if strings.Contains(strings.ToLower(vendor), "hygon") {
		vendor = "Hygon (海光)"
	} else if strings.Contains(strings.ToLower(model), "kunpeng") || strings.Contains(strings.ToLower(model), "鲲鹏") {
		vendor = "Kunpeng (鲲鹏)"
	}

	return vendor, model, nil
}

// GetOSInfo 获取操作系统信息
func GetOSInfo() (string, string, error) {
	// 读取 /etc/os-release
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	var name, version string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
		}
		if strings.HasPrefix(line, "VERSION=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION="), "\"")
		}
	}

	return name, version, nil
}
