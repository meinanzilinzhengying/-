/*
 * Cloud Flow Agent - libbpf Loader
 *
 * 使用 libbpf 加载和管理 eBPF 程序
 * 支持 CO-RE (Compile Once, Run Everywhere)
 */

package libbpf

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

// Loader libbpf 加载器
type Loader struct {
	mu sync.RWMutex

	// eBPF 集合
	collection *ebpf.Collection
	spec       *ebpf.CollectionSpec

	// 已加载的程序
	programs map[string]*ebpf.Program

	// 已加载的 maps
	maps map[string]*ebpf.Map

	// 已附加的链接
	links map[string]link.Link

	// Ring buffer reader
	ringReader *ringbuf.Reader
	perfReader *perf.Reader

	// 配置
	config *LoaderConfig

	// 事件回调
	eventHandler EventHandler

	// 运行状态
	running bool
	stopCh  chan struct{}
}

// LoaderConfig 加载器配置
type LoaderConfig struct {
	// BPF 对象文件路径
	ObjectPath string

	// BTF 文件路径（可选，用于 CO-RE）
	BTFPath string

	// 是否使用 CO-RE
	UseCORE bool

	// Map 大小覆盖
	MapSizes map[string]int

	// 程序附加类型覆盖
	ProgramTypes map[string]ebpf.ProgramType
}

// EventHandler 事件处理回调
type EventHandler func(data []byte)

// NewLoader 创建新的 libbpf 加载器
func NewLoader(config *LoaderConfig) (*Loader, error) {
	if config == nil {
		config = &LoaderConfig{}
	}

	// 解除 memlock 限制
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("failed to remove memlock limit: %w", err)
	}

	return &Loader{
		programs: make(map[string]*ebpf.Program),
		maps:     make(map[string]*ebpf.Map),
		links:    make(map[string]link.Link),
		config:   config,
		stopCh:   make(chan struct{}),
	}, nil
}

// Load 加载 eBPF 对象文件
func (l *Loader) Load(objectPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.collection != nil {
		return fmt.Errorf("bpf programs already loaded")
	}

	// 如果未指定路径，使用默认路径
	if objectPath == "" {
		objectPath = l.config.ObjectPath
	}

	// 加载 BTF（如果启用 CO-RE）
	var btfSpec *btf.Spec
	var err error
	if l.config.UseCORE {
		btfSpec, err = l.loadBTF()
		if err != nil {
			// CO-RE 不是必须的，记录警告继续
			fmt.Printf("Warning: BTF load failed: %v, continuing without CO-RE\n", err)
		}
	}

	// 加载 eBPF 对象文件
	spec, err := ebpf.LoadCollectionSpec(objectPath)
	if err != nil {
		return fmt.Errorf("failed to load collection spec: %w", err)
	}

	// 应用 Map 大小覆盖
	for name, size := range l.config.MapSizes {
		if m := spec.Maps[name]; m != nil {
			m.MaxEntries = uint32(size)
		}
	}

	// 应用程序类型覆盖
	for name, progType := range l.config.ProgramTypes {
		if p := spec.Programs[name]; p != nil {
			p.Type = progType
		}
	}

	// 加载选项
	opts := &ebpf.CollectionOptions{}
	if btfSpec != nil {
		opts.Programs.KernelTypes = btfSpec
	}

	// 加载集合
	coll, err := ebpf.NewCollectionWithOptions(spec, *opts)
	if err != nil {
		return fmt.Errorf("failed to load collection: %w", err)
	}

	l.collection = coll
	l.spec = spec

	// 提取 programs 和 maps
	for name, prog := range coll.Programs {
		l.programs[name] = prog
	}

	for name, m := range coll.Maps {
		l.maps[name] = m
	}

	return nil
}

// loadBTF 加载 BTF 信息
func (l *Loader) loadBTF() (*btf.Spec, error) {
	btfPath := l.config.BTFPath
	if btfPath == "" {
		// 尝试标准路径
		paths := []string{
			"/sys/kernel/btf/vmlinux",
			"/boot/vmlinux-$(uname -r)",
			"/lib/modules/$(uname -r)/build/vmlinux",
		}

		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				btfPath = p
				break
			}
		}
	}

	if btfPath == "" {
		return nil, fmt.Errorf("no BTF file found")
	}

	return btf.LoadSpec(btfPath)
}

// AttachKprobe 附加 kprobe
func (l *Loader) AttachKprobe(progName, symbol string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	prog := l.programs[progName]
	if prog == nil {
		return fmt.Errorf("program %s not found", progName)
	}

	// 尝试使用 fentry（性能更好）
	lnk, err := link.AttachTracing(link.TracingOptions{
		Program: prog,
	})
	if err != nil {
		// 回退到 kprobe
		lnk, err = link.Kprobe(symbol, prog, nil)
		if err != nil {
			return fmt.Errorf("failed to attach kprobe %s: %w", symbol, err)
		}
	}

	l.links[progName] = lnk
	return nil
}

// AttachKretprobe 附加 kretprobe
func (l *Loader) AttachKretprobe(progName, symbol string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	prog := l.programs[progName]
	if prog == nil {
		return fmt.Errorf("program %s not found", progName)
	}

	lnk, err := link.Kretprobe(symbol, prog, nil)
	if err != nil {
		return fmt.Errorf("failed to attach kretprobe %s: %w", symbol, err)
	}

	l.links[progName] = lnk
	return nil
}

// AttachTracepoint 附加 tracepoint
func (l *Loader) AttachTracepoint(progName, category, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	prog := l.programs[progName]
	if prog == nil {
		return fmt.Errorf("program %s not found", progName)
	}

	lnk, err := link.Tracepoint(category, name, prog, nil)
	if err != nil {
		return fmt.Errorf("failed to attach tracepoint %s:%s: %w", category, name, err)
	}

	l.links[progName] = lnk
	return nil
}

// AttachFentry 附加 fentry (BPF_TRACING)
func (l *Loader) AttachFentry(progName, target string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	prog := l.programs[progName]
	if prog == nil {
		return fmt.Errorf("program %s not found", progName)
	}

	lnk, err := link.AttachTracing(link.TracingOptions{
		Program:    prog,
		AttachType: ebpf.AttachTraceFEntry,
	})
	if err != nil {
		return fmt.Errorf("failed to attach fentry %s: %w", target, err)
	}

	l.links[progName] = lnk
	return nil
}

// AttachFexit 附加 fexit
func (l *Loader) AttachFexit(progName, target string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	prog := l.programs[progName]
	if prog == nil {
		return fmt.Errorf("program %s not found", progName)
	}

	lnk, err := link.AttachTracing(link.TracingOptions{
		Program:    prog,
		AttachType: ebpf.AttachTraceFExit,
	})
	if err != nil {
		return fmt.Errorf("failed to attach fexit %s: %w", target, err)
	}

	l.links[progName] = lnk
	return nil
}

// AttachRawTracepoint 附加 raw tracepoint
func (l *Loader) AttachRawTracepoint(progName, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	prog := l.programs[progName]
	if prog == nil {
		return fmt.Errorf("program %s not found", progName)
	}

	lnk, err := link.AttachRawTracepoint(link.RawTracepointOptions{
		Name:    name,
		Program: prog,
	})
	if err != nil {
		return fmt.Errorf("failed to attach raw tracepoint %s: %w", name, err)
	}

	l.links[progName] = lnk
	return nil
}

// Detach 分离指定程序
func (l *Loader) Detach(progName string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	lnk := l.links[progName]
	if lnk == nil {
		return fmt.Errorf("program %s not attached", progName)
	}

	if err := lnk.Close(); err != nil {
		return fmt.Errorf("failed to detach %s: %w", progName, err)
	}

	delete(l.links, progName)
	return nil
}

// DetachAll 分离所有程序
func (l *Loader) DetachAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error
	for name, lnk := range l.links {
		if err := lnk.Close(); err != nil {
			errs = append(errs, fmt.Errorf("detach %s: %w", name, err))
		}
		delete(l.links, name)
	}

	if len(errs) > 0 {
		return fmt.Errorf("detach errors: %v", errs)
	}
	return nil
}

// StartRingBuffer 启动 Ring Buffer 读取
func (l *Loader) StartRingBuffer(mapName string, handler EventHandler) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return fmt.Errorf("ring buffer already running")
	}

	m := l.maps[mapName]
	if m == nil {
		return fmt.Errorf("map %s not found", mapName)
	}

	reader, err := ringbuf.NewReader(m)
	if err != nil {
		return fmt.Errorf("failed to create ringbuf reader: %w", err)
	}

	l.ringReader = reader
	l.eventHandler = handler
	l.running = true

	// 启动读取 goroutine
	go l.ringBufferLoop()

	return nil
}

// ringBufferLoop Ring Buffer 读取循环
func (l *Loader) ringBufferLoop() {
	for {
		select {
		case <-l.stopCh:
			return
		default:
		}

		record, err := l.ringReader.Read()
		if err != nil {
			if err == ringbuf.ErrClosed {
				return
			}
			continue
		}

		if l.eventHandler != nil {
			l.eventHandler(record.RawSample)
		}
	}
}

// StopRingBuffer 停止 Ring Buffer 读取
func (l *Loader) StopRingBuffer() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return nil
	}

	close(l.stopCh)
	l.running = false

	if l.ringReader != nil {
		return l.ringReader.Close()
	}

	return nil
}

// UpdateMap 更新 Map 值
func (l *Loader) UpdateMap(mapName string, key, value unsafe.Pointer) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := l.maps[mapName]
	if m == nil {
		return fmt.Errorf("map %s not found", mapName)
	}

	return m.Update(key, value, ebpf.UpdateAny)
}

// LookupMap 查询 Map 值
func (l *Loader) LookupMap(mapName string, key, value unsafe.Pointer) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := l.maps[mapName]
	if m == nil {
		return fmt.Errorf("map %s not found", mapName)
	}

	return m.Lookup(key, value)
}

// DeleteMapKey 删除 Map 键
func (l *Loader) DeleteMapKey(mapName string, key unsafe.Pointer) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := l.maps[mapName]
	if m == nil {
		return fmt.Errorf("map %s not found", mapName)
	}

	return m.Delete(key)
}

// GetMap 获取 Map 对象（用于高级操作）
func (l *Loader) GetMap(name string) *ebpf.Map {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.maps[name]
}

// GetProgram 获取 Program 对象
func (l *Loader) GetProgram(name string) *ebpf.Program {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.programs[name]
}

// GetCollection 获取整个 Collection
func (l *Loader) GetCollection() *ebpf.Collection {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.collection
}

// ListPrograms 列出所有已加载的程序
func (l *Loader) ListPrograms() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.programs))
	for name := range l.programs {
		names = append(names, name)
	}
	return names
}

// ListMaps 列出所有已加载的 maps
func (l *Loader) ListMaps() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.maps))
	for name := range l.maps {
		names = append(names, name)
	}
	return names
}

// ListAttached 列出所有已附加的程序
func (l *Loader) ListAttached() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.links))
	for name := range l.links {
		names = append(names, name)
	}
	return names
}

// Close 关闭加载器并清理资源
func (l *Loader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error

	// 停止 ring buffer
	if l.running {
		close(l.stopCh)
		l.running = false
	}

	if l.ringReader != nil {
		if err := l.ringReader.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// 分离所有链接
	for name, lnk := range l.links {
		if err := lnk.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close link %s: %w", name, err))
		}
	}
	l.links = make(map[string]link.Link)

	// 关闭集合（会自动关闭所有 programs 和 maps）
	if l.collection != nil {
		l.collection.Close()
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// IsAttached 检查程序是否已附加
func (l *Loader) IsAttached(progName string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.links[progName]
	return ok
}

// GetProgramInfo 获取程序信息
func (l *Loader) GetProgramInfo(progName string) (*ebpf.ProgramInfo, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	prog := l.programs[progName]
	if prog == nil {
		return nil, fmt.Errorf("program %s not found", progName)
	}

	return prog.Info()
}

// GetMapInfo 获取 Map 信息
func (l *Loader) GetMapInfo(mapName string) (*ebpf.MapInfo, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := l.maps[mapName]
	if m == nil {
		return nil, fmt.Errorf("map %s not found", mapName)
	}

	return m.Info()
}

// PinMap 将 Map 固定到 BPF 文件系统
func (l *Loader) PinMap(mapName, path string) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := l.maps[mapName]
	if m == nil {
		return fmt.Errorf("map %s not found", mapName)
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return m.Pin(path)
}

// UnpinMap 从 BPF 文件系统取消固定 Map
func (l *Loader) UnpinMap(mapName string) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m := l.maps[mapName]
	if m == nil {
		return fmt.Errorf("map %s not found", mapName)
	}

	return m.Unpin()
}

// LoadPinnedMap 加载已固定的 Map
func LoadPinnedMap(path string) (*ebpf.Map, error) {
	return ebpf.LoadPinnedMap(path, nil)
}

// LoadPinnedProgram 加载已固定的 Program
func LoadPinnedProgram(path string) (*ebpf.Program, error) {
	return ebpf.LoadPinnedProgram(path, nil)
}
