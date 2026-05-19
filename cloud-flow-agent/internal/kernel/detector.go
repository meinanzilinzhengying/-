// Package kernel 提供内核能力检测功能
// 用于检测系统架构、内核版本、eBPF/BTF/RingBuffer 支持以及国产芯片（海光/鲲鹏）
package kernel

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"cloud-flow-agent/pkg/logger"
)

// Arch 架构类型
type Arch string

const (
	ArchX86_64  Arch = "x86_64"
	ArchAARCH64 Arch = "aarch64"
	ArchUnknown Arch = "unknown"
)

// Vendor 芯片厂商
type Vendor string

const (
	VendorIntel   Vendor = "intel"
	VendorAMD     Vendor = "amd"
	VendorHygon   Vendor = "hygon"   // 海光
	VendorKunpeng Vendor = "kunpeng" // 鲲鹏
	VendorUnknown Vendor = "unknown"
)

// KernelCapability 内核能力检测结果
type KernelCapability struct {
	Arch            Arch              // 系统架构
	Vendor          Vendor            // 芯片厂商
	KernelVersion   string            // 内核版本（完整字符串）
	KernelMajor     int               // 内核主版本号
	KernelMinor     int               // 内核次版本号
	KernelPatch     int               // 内核补丁版本号
	EBPFSupported   bool              // 是否支持 eBPF
	BTFSupported    bool              // 是否支持 BTF
	RingBufSupported bool             // 是否支持 BPF RingBuffer
	Capabilities    map[string]bool   // 详细能力清单
}

// Detector 内核能力检测器
type Detector struct {
	log    *logger.Logger
	mu     sync.RWMutex
	result *KernelCapability
}

// NewDetector 创建内核能力检测器
func NewDetector(log *logger.Logger) *Detector {
	return &Detector{
		log: log,
	}
}

// Detect 执行内核能力检测
func (d *Detector) Detect() (*KernelCapability, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cap := &KernelCapability{
		Capabilities: make(map[string]bool),
	}

	// 1. 检测系统架构
	cap.Arch = detectArch()
	d.log.Infof("检测到系统架构: %s", cap.Arch)

	// 2. 检测芯片厂商
	cap.Vendor = detectVendor(cap.Arch)
	d.log.Infof("检测到芯片厂商: %s", cap.Vendor)

	// 3. 检测内核版本
	kernelVersion, major, minor, patch, err := detectKernelVersion()
	if err != nil {
		d.log.Warnf("检测内核版本失败: %v", err)
		return nil, fmt.Errorf("检测内核版本失败: %w", err)
	}
	cap.KernelVersion = kernelVersion
	cap.KernelMajor = major
	cap.KernelMinor = minor
	cap.KernelPatch = patch
	d.log.Infof("检测到内核版本: %s (major=%d, minor=%d, patch=%d)", kernelVersion, major, minor, patch)

	// 4. 检测 eBPF 支持
	cap.EBPFSupported = detectEBPFSupport(major, minor)
	cap.Capabilities["ebpf"] = cap.EBPFSupported
	d.log.Infof("eBPF 支持: %v", cap.EBPFSupported)

	// 5. 检测 BTF 支持
	cap.BTFSupported = detectBTFSupport()
	cap.Capabilities["btf"] = cap.BTFSupported
	d.log.Infof("BTF 支持: %v", cap.BTFSupported)

	// 6. 检测 BPF RingBuffer 支持（内核 >= 5.8）
	cap.RingBufSupported = detectRingBufSupport(major, minor)
	cap.Capabilities["ringbuf"] = cap.RingBufSupported
	d.log.Infof("BPF RingBuffer 支持: %v", cap.RingBufSupported)

	// 7. 检测其他能力
	cap.Capabilities["bpf_perf_event"] = detectBPFPerfEvent()
	cap.Capabilities["bpf_cgroup"] = detectBPFCgroup(major, minor)
	cap.Capabilities["bpf_tracepoint"] = detectBPFTracepoint()
	cap.Capabilities["kprobes"] = detectKprobes()
	cap.Capabilities["uprobes"] = detectUprobes()

	d.result = cap
	return cap, nil
}

// GetResult 获取缓存的检测结果
func (d *Detector) GetResult() *KernelCapability {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.result
}

// Summary 返回检测结果的摘要字符串
func (c *KernelCapability) Summary() string {
	vendorStr := string(c.Vendor)
	if c.Vendor == VendorHygon {
		vendorStr = "海光 (Hygon)"
	} else if c.Vendor == VendorKunpeng {
		vendorStr = "鲲鹏 (Kunpeng)"
	}

	return fmt.Sprintf("Arch=%s, Vendor=%s, Kernel=%s, eBPF=%v, BTF=%v, RingBuf=%v",
		c.Arch, vendorStr, c.KernelVersion, c.EBPFSupported, c.BTFSupported, c.RingBufSupported)
}

// IsDomesticChip 是否为国产芯片
func (c *KernelCapability) IsDomesticChip() bool {
	return c.Vendor == VendorHygon || c.Vendor == VendorKunpeng
}

// IsEBPFAvailable eBPF 是否可用（综合考虑 eBPF 和 BTF 支持）
func (c *KernelCapability) IsEBPFAvailable() bool {
	return c.EBPFSupported
}

// detectArch 检测系统架构
func detectArch() Arch {
	goArch := runtime.GOARCH
	switch goArch {
	case "amd64":
		return ArchX86_64
	case "arm64":
		return ArchAARCH64
	default:
		return Arch(goArch)
	}
}

// detectVendor 检测芯片厂商
func detectVendor(arch Arch) Vendor {
	if arch == ArchAARCH64 {
		// ARM64 架构，检测是否为鲲鹏
		if isKunpeng() {
			return VendorKunpeng
		}
		return VendorUnknown
	}

	if arch == ArchX86_64 {
		// x86_64 架构，通过 CPUID 检测厂商
		vendor := detectX86Vendor()
		if vendor == VendorUnknown {
			// 尝试通过 /proc/cpuinfo 检测海光
			if isHygon() {
				return VendorHygon
			}
		}
		return vendor
	}

	return VendorUnknown
}

// detectX86Vendor 通过 /proc/cpuinfo 检测 x86 芯片厂商
func detectX86Vendor() Vendor {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return VendorUnknown
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "vendor_id") {
			fields := strings.SplitN(line, ":", 2)
			if len(fields) == 2 {
				vendorID := strings.TrimSpace(fields[1])
				switch vendorID {
				case "GenuineIntel":
					return VendorIntel
				case "AuthenticAMD":
					return VendorAMD
				case "HygonGenuine":
					return VendorHygon
				}
			}
		}
		// model name 中也可能包含厂商信息
		if strings.HasPrefix(line, "model name") {
			fields := strings.SplitN(line, ":", 2)
			if len(fields) == 2 {
				modelName := strings.ToLower(fields[1])
				if strings.Contains(modelName, "hygon") {
					return VendorHygon
				}
			}
		}
	}

	return VendorUnknown
}

// isHygon 检测是否为海光芯片（通过 /proc/cpuinfo 的 model name）
func isHygon() bool {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			fields := strings.SplitN(line, ":", 2)
			if len(fields) == 2 {
				modelName := strings.ToLower(fields[1])
				return strings.Contains(modelName, "hygon") ||
					strings.Contains(modelName, "dhyana") ||
					strings.Contains(modelName, "海光")
			}
		}
	}
	return false
}

// isKunpeng 检测是否为鲲鹏芯片（通过 /proc/cpuinfo 的 implementer 字段或 model name）
func isKunpeng() bool {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// ARM64 的 CPU implementer 字段，华为/鲲鹏的 implementer 为 0x48 (H)
		if strings.HasPrefix(line, "CPU implementer") {
			fields := strings.SplitN(line, ":", 2)
			if len(fields) == 2 {
				impl := strings.TrimSpace(fields[1])
				// 0x48 是华为的 implementer ID
				if impl == "0x48" || impl == "0x41" {
					// 进一步检查 model name
					continue // 继续检查 model name
				}
			}
		}
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Hardware") {
			fields := strings.SplitN(line, ":", 2)
			if len(fields) == 2 {
				value := strings.ToLower(fields[1])
				if strings.Contains(value, "kunpeng") ||
					strings.Contains(value, "鲲鹏") ||
					strings.Contains(value, "hi silicon") ||
					strings.Contains(value, "hisilicon") {
					return true
				}
			}
		}
	}
	return false
}

// detectKernelVersion 检测内核版本
func detectKernelVersion() (version string, major, minor, patch int, err error) {
	var unameBuf syscall.Utsname

	// 使用 syscall.Uname 获取内核版本
	if errno := syscall.Uname(&unameBuf); errno != nil {
		// 回退到读取 /proc/version
		return detectKernelVersionFromProc()
	}

	release := charsToString(unameBuf.Release[:])
	return parseKernelVersion(release)
}

// charsToString 将 [65]int8 数组转换为字符串（处理 NUL 终止）
func charsToString(arr []int8) string {
	var buf []byte
	for _, c := range arr {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
	}
	return string(buf)
}

// detectKernelVersionFromProc 从 /proc/version 解析内核版本
func detectKernelVersionFromProc() (version string, major, minor, patch int, err error) {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("读取 /proc/version 失败: %w", err)
	}

	// /proc/version 格式: Linux version 5.15.0-91-generic ...
	content := string(data)
	idx := strings.Index(content, "Linux version ")
	if idx == -1 {
		return "", 0, 0, 0, fmt.Errorf("无法解析 /proc/version")
	}

	rest := content[idx+len("Linux version "):]
	end := strings.IndexAny(rest, " \t")
	if end == -1 {
		end = len(rest)
	}
	version = rest[:end]

	return parseKernelVersion(version)
}

// parseKernelVersion 解析内核版本字符串
func parseKernelVersion(version string) (string, int, int, int, error) {
	// 移除可能的发行版后缀，如 "-generic"、".el7" 等
	// 只取前三个数字部分
	parts := strings.SplitN(version, "-", 2)
	versionCore := parts[0]

	// 解析 major.minor.patch
	versionParts := strings.SplitN(versionCore, ".", 4)
	if len(versionParts) < 3 {
		return version, 0, 0, 0, fmt.Errorf("无法解析内核版本: %s", version)
	}

	major, err1 := strconv.Atoi(versionParts[0])
	minor, err2 := strconv.Atoi(versionParts[1])
	patch, err3 := strconv.Atoi(versionParts[2])

	if err1 != nil || err2 != nil || err3 != nil {
		return version, 0, 0, 0, fmt.Errorf("无法解析内核版本号: %s", version)
	}

	return version, major, minor, patch, nil
}

// detectEBPFSupport 检测 eBPF 支持
// 内核 >= 4.10 支持 eBPF 程序加载
func detectEBPFSupport(major, minor int) bool {
	if major > 4 {
		return true
	}
	if major == 4 && minor >= 10 {
		return true
	}

	// 额外检查：尝试打开 bpf 文件系统
	if _, err := os.Stat("/sys/fs/bpf"); err == nil {
		return true
	}

	return false
}

// detectBTFSupport 检测 BTF (BPF Type Format) 支持
// 通过检查 /sys/kernel/btf/vmlinux 是否存在
func detectBTFSupport() bool {
	_, err := os.Stat("/sys/kernel/btf/vmlinux")
	return err == nil
}

// detectRingBufSupport 检测 BPF RingBuffer 支持
// 内核 >= 5.8 支持 BPF RingBuffer
func detectRingBufSupport(major, minor int) bool {
	if major > 5 {
		return true
	}
	if major == 5 && minor >= 8 {
		return true
	}
	return false
}

// detectBPFPerfEvent 检测 BPF perf event 支持
func detectBPFPerfEvent() bool {
	// 检查 perf_event 是否可用
	f, err := os.Open("/sys/kernel/debug/tracing/available_events")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// detectBPFCgroup 检测 BPF cgroup 支持
// 内核 >= 4.10 支持 cgroup BPF
func detectBPFCgroup(major, minor int) bool {
	if major > 4 {
		return true
	}
	if major == 4 && minor >= 10 {
		return true
	}
	return false
}

// detectBPFTracepoint 检测 BPF tracepoint 支持
func detectBPFTracepoint() bool {
	_, err := os.Stat("/sys/kernel/debug/tracing/available_events")
	return err == nil
}

// detectKprobes 检测 kprobes 支持
func detectKprobes() bool {
	data, err := os.ReadFile("/sys/kernel/debug/kprobes/enabled")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

// detectUprobes 检测 uprobes 支持
func detectUprobes() bool {
	data, err := os.ReadFile("/sys/kernel/debug/uprobes/enabled")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}
