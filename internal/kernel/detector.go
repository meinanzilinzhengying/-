// Package kernel 内核与架构兼容性检测
// 支持 x86/ARM/鲲鹏/海光，内核版本校验，低内核自动降级
package kernel

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ============================================================
// 架构类型定义
// ============================================================

// ArchType 系统架构类型
type ArchType string

const (
	ArchX86_64   ArchType = "x86_64"   // x86 64位 (Intel/AMD)
	ArchARM64    ArchType = "aarch64"  // ARM 64位
	ArchKunpeng  ArchType = "kunpeng"  // 鲲鹏 (ARM架构)
	ArchHygon    ArchType = "hygon"    // 海光 (x86架构)
	ArchUnknown  ArchType = "unknown"
)

// String 返回架构名称
func (a ArchType) String() string {
	switch a {
	case ArchX86_64:
		return "x86_64 (Intel/AMD)"
	case ArchARM64:
		return "aarch64 (ARM64)"
	case ArchKunpeng:
		return "Kunpeng (鲲鹏)"
	case ArchHygon:
		return "Hygon (海光)"
	default:
		return "Unknown"
	}
}

// IsSupported 检查架构是否受支持
func (a ArchType) IsSupported() bool {
	switch a {
	case ArchX86_64, ArchARM64, ArchKunpeng, ArchHygon:
		return true
	default:
		return false
	}
}

// IsX86 是否为x86架构
func (a ArchType) IsX86() bool {
	return a == ArchX86_64 || a == ArchHygon
}

// IsARM 是否为ARM架构
func (a ArchType) IsARM() bool {
	return a == ArchARM64 || a == ArchKunpeng
}

// ============================================================
// 内核能力
// ============================================================

// KernelCapability 内核能力检测
type KernelCapability struct {
	Version         string    `json:"version"`          // 内核版本字符串
	Major           int       `json:"major"`            // 主版本号
	Minor           int       `json:"minor"`            // 次版本号
	Patch           int       `json:"patch"`            // 补丁版本号
	Arch            ArchType  `json:"arch"`             // 系统架构
	SupportsEBPF    bool      `json:"supports_ebpf"`    // 是否支持 eBPF
	SupportsBTF     bool      `json:"supports_btf"`     // 是否支持 BTF
	SupportsRingBuf bool      `json:"supports_ringbuf"` // 是否支持 Ring Buffer
	SupportsKprobe  bool      `json:"supports_kprobe"`  // 是否支持 Kprobe
	SupportsUprobe  bool      `json:"supports_uprobe"`  // 是否支持 Uprobe
	SupportsTracepoint bool   `json:"supports_tracepoint"` // 是否支持 Tracepoint
	MinRequired     bool      `json:"min_required"`     // 是否满足最低要求
	DetectedAt      int64     `json:"detected_at"`      // 检测时间戳
}

// FeatureLevel 功能等级
type FeatureLevel int

const (
	FeatureLevelNone     FeatureLevel = iota // 无eBPF支持
	FeatureLevelBasic                        // 基础eBPF (kernel >= 4.1)
	FeatureLevelEnhanced                     // 增强eBPF (kernel >= 4.14)
	FeatureLevelAdvanced                     // 高级eBPF (kernel >= 5.2)
	FeatureLevelFull                         // 完整eBPF (kernel >= 5.8)
)

// String 返回功能等级描述
func (f FeatureLevel) String() string {
	switch f {
	case FeatureLevelNone:
		return "None (无eBPF支持)"
	case FeatureLevelBasic:
		return "Basic (基础eBPF)"
	case FeatureLevelEnhanced:
		return "Enhanced (增强eBPF)"
	case FeatureLevelAdvanced:
		return "Advanced (高级eBPF)"
	case FeatureLevelFull:
		return "Full (完整eBPF)"
	default:
		return "Unknown"
	}
}

// ============================================================
// 检测器
// ============================================================

// Detector 内核检测器
type Detector struct {
	capability *KernelCapability
}

// NewDetector 创建检测器
func NewDetector() *Detector {
	return &Detector{}
}

// Detect 执行检测
func (d *Detector) Detect() (*KernelCapability, error) {
	cap := &KernelCapability{
		DetectedAt: time.Now().Unix(),
	}

	// 检测架构
	cap.Arch = d.detectArch()
	if !cap.Arch.IsSupported() {
		return cap, fmt.Errorf("不支持的架构: %s", cap.Arch)
	}

	// 检测内核版本
	version, major, minor, patch, err := d.detectKernelVersion()
	if err != nil {
		return cap, fmt.Errorf("检测内核版本失败: %w", err)
	}
	cap.Version = version
	cap.Major = major
	cap.Minor = minor
	cap.Patch = patch

	// 检查最低版本要求 (3.10+)
	cap.MinRequired = d.checkMinRequirement(major, minor)

	// 检测eBPF支持
	cap.SupportsEBPF = d.checkEBPFSupport(major, minor)
	cap.SupportsBTF = d.checkBTFSupport(major, minor)
	cap.SupportsRingBuf = d.checkRingBufSupport(major, minor)
	cap.SupportsKprobe = d.checkKprobeSupport(major, minor)
	cap.SupportsUprobe = d.checkUprobeSupport(major, minor)
	cap.SupportsTracepoint = d.checkTracepointSupport(major, minor)

	d.capability = cap
	return cap, nil
}

// detectArch 检测系统架构
func (d *Detector) detectArch() ArchType {
	// 首先检查runtime.GOARCH
	goarch := runtime.GOARCH
	
	// 读取 /proc/cpuinfo 获取更详细的信息
	cpuinfo, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		// 降级使用GOARCH
		return d.mapGOARCH(goarch)
	}
	
	cpuinfoStr := string(cpuinfo)
	
	// 检测鲲鹏 (Kunpeng)
	if strings.Contains(cpuinfoStr, "Kunpeng") || 
	   strings.Contains(cpuinfoStr, "HUAWEI") ||
	   strings.Contains(cpuinfoStr, "kunpeng") {
		return ArchKunpeng
	}
	
	// 检测海光 (Hygon)
	if strings.Contains(cpuinfoStr, "Hygon") || 
	   strings.Contains(cpuinfoStr, "hygon") {
		return ArchHygon
	}
	
	// 检测ARM64
	if strings.Contains(cpuinfoStr, "ARM") || 
	   strings.Contains(cpuinfoStr, "aarch64") ||
	   goarch == "arm64" {
		return ArchARM64
	}
	
	// 检测x86_64
	if strings.Contains(cpuinfoStr, "x86_64") || 
	   strings.Contains(cpuinfoStr, "Intel") ||
	   strings.Contains(cpuinfoStr, "AMD") ||
	   goarch == "amd64" {
		return ArchX86_64
	}
	
	return ArchUnknown
}

// mapGOARCH 映射GOARCH到ArchType
func (d *Detector) mapGOARCH(goarch string) ArchType {
	switch goarch {
	case "amd64":
		return ArchX86_64
	case "arm64":
		return ArchARM64
	default:
		return ArchUnknown
	}
}

// detectKernelVersion 检测内核版本
func (d *Detector) detectKernelVersion() (string, int, int, int, error) {
	// 方法1: 使用 syscall.Utsname
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err == nil {
		version := utsnameToString(uname.Release)
		major, minor, patch := parseVersion(version)
		return version, major, minor, patch, nil
	}
	
	// 方法2: 读取 /proc/version
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "", 0, 0, 0, err
	}
	
	// 解析版本字符串
	// 格式: Linux version 5.15.0-105-generic ...
	re := regexp.MustCompile(`Linux version (\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) >= 4 {
		major, _ := strconv.Atoi(matches[1])
		minor, _ := strconv.Atoi(matches[2])
		patch, _ := strconv.Atoi(matches[3])
		version := fmt.Sprintf("%d.%d.%d", major, minor, patch)
		return version, major, minor, patch, nil
	}
	
	return "", 0, 0, 0, fmt.Errorf("无法解析内核版本")
}

// checkMinRequirement 检查最低版本要求
func (d *Detector) checkMinRequirement(major, minor int) bool {
	// 最低要求: Linux 3.10
	if major > 3 {
		return true
	}
	if major == 3 && minor >= 10 {
		return true
	}
	return false
}

// checkEBPFSupport 检查eBPF支持
func (d *Detector) checkEBPFSupport(major, minor int) bool {
	// eBPF 需要 kernel >= 3.18
	// 但完整功能需要 >= 4.1
	if major > 4 {
		return true
	}
	if major == 4 && minor >= 1 {
		return true
	}
	if major == 3 && minor >= 18 {
		return true // 基础支持
	}
	return false
}

// checkBTFSupport 检查BTF支持
func (d *Detector) checkBTFSupport(major, minor int) bool {
	// BTF 需要 kernel >= 5.2
	if major > 5 {
		return true
	}
	if major == 5 && minor >= 2 {
		return true
	}
	return false
}

// checkRingBufSupport 检查Ring Buffer支持
func (d *Detector) checkRingBufSupport(major, minor int) bool {
	// Ring Buffer 需要 kernel >= 5.8
	if major > 5 {
		return true
	}
	if major == 5 && minor >= 8 {
		return true
	}
	return false
}

// checkKprobeSupport 检查Kprobe支持
func (d *Detector) checkKprobeSupport(major, minor int) bool {
	// Kprobe 需要 kernel >= 4.1
	if major > 4 {
		return true
	}
	if major == 4 && minor >= 1 {
		return true
	}
	return false
}

// checkUprobeSupport 检查Uprobe支持
func (d *Detector) checkUprobeSupport(major, minor int) bool {
	// Uprobe 需要 kernel >= 4.17
	if major > 4 {
		return true
	}
	if major == 4 && minor >= 17 {
		return true
	}
	return false
}

// checkTracepointSupport 检查Tracepoint支持
func (d *Detector) checkTracepointSupport(major, minor int) bool {
	// Tracepoint 需要 kernel >= 4.7
	if major > 4 {
		return true
	}
	if major == 4 && minor >= 7 {
		return true
	}
	return false
}

// GetFeatureLevel 获取功能等级
func (d *Detector) GetFeatureLevel() FeatureLevel {
	if d.capability == nil {
		return FeatureLevelNone
	}
	
	major := d.capability.Major
	minor := d.capability.Minor
	
	if major >= 5 && minor >= 8 {
		return FeatureLevelFull
	}
	if major >= 5 && minor >= 2 {
		return FeatureLevelAdvanced
	}
	if major >= 4 && minor >= 14 {
		return FeatureLevelEnhanced
	}
	if major >= 4 && minor >= 1 {
		return FeatureLevelBasic
	}
	return FeatureLevelNone
}

// GetCapability 获取检测到的能力
func (d *Detector) GetCapability() *KernelCapability {
	return d.capability
}

// ============================================================
// 兼容性报告
// ============================================================

// CompatibilityReport 兼容性报告
type CompatibilityReport struct {
	Compatible      bool     `json:"compatible"`
	Arch            ArchType `json:"arch"`
	KernelVersion   string   `json:"kernel_version"`
	FeatureLevel    string   `json:"feature_level"`
	SupportedFeatures []string `json:"supported_features"`
	MissingFeatures []string `json:"missing_features,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// GenerateReport 生成兼容性报告
func (d *Detector) GenerateReport() *CompatibilityReport {
	if d.capability == nil {
		return &CompatibilityReport{
			Compatible: false,
			Recommendations: []string{"请先执行内核检测"},
		}
	}
	
	cap := d.capability
	report := &CompatibilityReport{
		Compatible:    cap.MinRequired && cap.SupportsEBPF,
		Arch:          cap.Arch,
		KernelVersion: cap.Version,
		FeatureLevel:  d.GetFeatureLevel().String(),
	}
	
	// 支持的功能
	if cap.SupportsEBPF {
		report.SupportedFeatures = append(report.SupportedFeatures, "eBPF")
	}
	if cap.SupportsBTF {
		report.SupportedFeatures = append(report.SupportedFeatures, "BTF")
	}
	if cap.SupportsRingBuf {
		report.SupportedFeatures = append(report.SupportedFeatures, "Ring Buffer")
	}
	if cap.SupportsKprobe {
		report.SupportedFeatures = append(report.SupportedFeatures, "Kprobe")
	}
	if cap.SupportsUprobe {
		report.SupportedFeatures = append(report.SupportedFeatures, "Uprobe")
	}
	if cap.SupportsTracepoint {
		report.SupportedFeatures = append(report.SupportedFeatures, "Tracepoint")
	}
	
	// 缺失的功能和建议
	if !cap.MinRequired {
		report.MissingFeatures = append(report.MissingFeatures, "内核版本过低")
		report.Recommendations = append(report.Recommendations, 
			fmt.Sprintf("内核版本 %s 低于最低要求 3.10，请升级内核", cap.Version))
	}
	
	if !cap.SupportsEBPF {
		report.MissingFeatures = append(report.MissingFeatures, "eBPF")
		report.Recommendations = append(report.Recommendations, 
			"当前内核不支持eBPF，将使用传统采集方式")
	}
	
	if !cap.SupportsBTF {
		report.MissingFeatures = append(report.MissingFeatures, "BTF")
		report.Recommendations = append(report.Recommendations, 
			"建议升级内核到5.2+以获得BTF支持，简化eBPF程序部署")
	}
	
	if !cap.SupportsRingBuf {
		report.MissingFeatures = append(report.MissingFeatures, "Ring Buffer")
		report.Recommendations = append(report.Recommendations, 
			"建议升级内核到5.8+以获得Ring Buffer支持，提升高流量场景性能")
	}
	
	return report
}

// ============================================================
// 工具函数
// ============================================================

// utsnameToString 将Utsname数组转换为字符串
func utsnameToString(arr [65]int8) string {
	var buf [65]byte
	for i, b := range arr {
		if b == 0 {
			break
		}
		buf[i] = byte(b)
	}
	return string(buf[:])
}

// parseVersion 解析版本字符串
func parseVersion(version string) (int, int, int) {
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) >= 4 {
		major, _ := strconv.Atoi(matches[1])
		minor, _ := strconv.Atoi(matches[2])
		patch, _ := strconv.Atoi(matches[3])
		return major, minor, patch
	}
	return 0, 0, 0
}

// ValidateBeforeStart 启动前验证
func ValidateBeforeStart() error {
	detector := NewDetector()
	cap, err := detector.Detect()
	if err != nil {
		return fmt.Errorf("内核检测失败: %w", err)
	}
	
	if !cap.Arch.IsSupported() {
		return fmt.Errorf("不支持的架构: %s", cap.Arch)
	}
	
	if !cap.MinRequired {
		return fmt.Errorf("内核版本 %s 低于最低要求 3.10", cap.Version)
	}
	
	return nil
}

// GetCollectorType 根据内核能力获取推荐的采集器类型
func GetCollectorType() string {
	detector := NewDetector()
	cap, err := detector.Detect()
	if err != nil {
		return "traditional" // 降级到传统采集器
	}
	
	if !cap.SupportsEBPF {
		return "traditional"
	}
	
	if cap.SupportsRingBuf {
		return "ebpf_advanced"
	}
	
	return "ebpf_basic"
}
