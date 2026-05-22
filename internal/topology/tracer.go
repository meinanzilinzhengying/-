// Package topology 路径追踪模块
// 支持请求在 Pod/VM/物理机 之间的路径追踪
package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// TracePath 路径追踪
type TracePath struct {
	ID           string          `json:"id" yaml:"id"`
	TraceID      string          `json:"trace_id" yaml:"trace_id"`           // 分布式追踪ID
	StartTime    time.Time       `json:"start_time" yaml:"start_time"`
	EndTime      time.Time       `json:"end_time" yaml:"end_time"`
	Duration     time.Duration   `json:"duration" yaml:"duration"`
	Source       *TraceEndpoint  `json:"source" yaml:"source"`               // 源端点
	Destination  *TraceEndpoint  `json:"destination" yaml:"destination"`     // 目标端点
	Hops         []*TraceHop     `json:"hops" yaml:"hops"`                   // 跳跃节点
	Status       string          `json:"status" yaml:"status"`               // success/failed/timeout
	ErrorCode    string          `json:"error_code" yaml:"error_code"`
	ErrorMessage string          `json:"error_message" yaml:"error_message"`
	Protocol     string          `json:"protocol" yaml:"protocol"`           // TCP/UDP/HTTP
	Method       string          `json:"method" yaml:"method"`               // HTTP方法
	Path         string          `json:"path" yaml:"path"`                   // HTTP路径
	StatusCode   int             `json:"status_code" yaml:"status_code"`     // HTTP状态码
	BytesSent    uint64          `json:"bytes_sent" yaml:"bytes_sent"`
	BytesRecv    uint64          `json:"bytes_recv" yaml:"bytes_recv"`
	Labels       map[string]string `json:"labels" yaml:"labels"`
}

// TraceEndpoint 追踪端点
type TraceEndpoint struct {
	EntityID     string            `json:"entity_id" yaml:"entity_id"`         // 关联的实体ID
	EntityType   EntityType        `json:"entity_type" yaml:"entity_type"`     // 实体类型
	EntityName   string            `json:"entity_name" yaml:"entity_name"`     // 实体名称
	IP           string            `json:"ip" yaml:"ip"`                       // IP地址
	Port         uint16            `json:"port" yaml:"port"`                   // 端口
	ProcessName  string            `json:"process_name" yaml:"process_name"`   // 进程名
	ProcessPID   uint32            `json:"process_pid" yaml:"process_pid"`     // 进程ID
	ContainerID  string            `json:"container_id" yaml:"container_id"`   // 容器ID
	PodName      string            `json:"pod_name" yaml:"pod_name"`           // Pod名称
	Namespace    string            `json:"namespace" yaml:"namespace"`         // K8s命名空间
	NodeName     string            `json:"node_name" yaml:"node_name"`         // 节点名
	ServiceName  string            `json:"service_name" yaml:"service_name"`   // 服务名
	Labels       map[string]string `json:"labels" yaml:"labels"`
}

// TraceHop 跳跃节点
type TraceHop struct {
	ID           int              `json:"id" yaml:"id"`                       // 跳跃序号
	EntityID     string           `json:"entity_id" yaml:"entity_id"`         // 实体ID
	EntityType   EntityType       `json:"entity_type" yaml:"entity_type"`     // 实体类型
	EntityName   string           `json:"entity_name" yaml:"entity_name"`     // 实体名称
	IP           string           `json:"ip" yaml:"ip"`                       // IP地址
	Port         uint16           `json:"port" yaml:"port"`                   // 端口
	InboundTime  time.Time        `json:"inbound_time" yaml:"inbound_time"`   // 入站时间
	OutboundTime time.Time        `json:"outbound_time" yaml:"outbound_time"` // 出站时间
	Duration     time.Duration    `json:"duration" yaml:"duration"`           // 处理耗时
	ProcessName  string           `json:"process_name" yaml:"process_name"`   // 进程名
	ProcessPID   uint32           `json:"process_pid" yaml:"process_pid"`     // 进程ID
	Action       string           `json:"action" yaml:"action"`               // 动作: forward/proxy/redirect
	Latency      time.Duration    `json:"latency" yaml:"latency"`             // 延迟
	Labels       map[string]string `json:"labels" yaml:"labels"`
}

// TracerConfig 追踪配置
type TracerConfig struct {
	Enabled           bool          `json:"enabled" yaml:"enabled"`
	SampleRate        float64       `json:"sample_rate" yaml:"sample_rate"`           // 采样率 (0-1)
	MaxPaths          int           `json:"max_paths" yaml:"max_paths"`               // 最大路径缓存数
	MaxHops           int           `json:"max_hops" yaml:"max_hops"`                 // 最大跳跃数
	PathTimeout       time.Duration `json:"path_timeout" yaml:"path_timeout"`         // 路径超时
	EnableE2E         bool          `json:"enable_e2e" yaml:"enable_e2e"`             // 启用端到端追踪
	EnableHopTracking bool          `json:"enable_hop_tracking" yaml:"enable_hop_tracking"` // 启用跳跃追踪
	EnableLatency     bool          `json:"enable_latency" yaml:"enable_latency"`     // 启用延迟追踪
	EnableDNS         bool          `json:"enable_dns" yaml:"enable_dns"`             // 启用DNS解析
	StoreRawPackets   bool          `json:"store_raw_packets" yaml:"store_raw_packets"` // 存储原始包
}

// PathTracer 路径追踪器
type PathTracer struct {
	config        *TracerConfig
	discovery     *DiscoveryEngine
	paths         map[string]*TracePath
	activePaths   map[string]*TracePath
	entityCache   map[string]*Entity
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	handlers      []TraceHandler
}

// TraceHandler 追踪事件处理器
type TraceHandler interface {
	OnPathStart(path *TracePath)
	OnPathComplete(path *TracePath)
	OnHopDiscovered(path *TracePath, hop *TraceHop)
}

// NewPathTracer 创建路径追踪器
func NewPathTracer(config *TracerConfig, discovery *DiscoveryEngine) *PathTracer {
	ctx, cancel := context.WithCancel(context.Background())
	
	tracer := &PathTracer{
		config:      config,
		discovery:   discovery,
		paths:       make(map[string]*TracePath),
		activePaths: make(map[string]*TracePath),
		entityCache: make(map[string]*Entity),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// 注册发现事件处理器
	if discovery != nil {
		discovery.RegisterHandler(&tracerDiscoveryHandler{tracer: tracer})
	}
	
	return tracer
}

// tracerDiscoveryHandler 发现事件处理器适配器
type tracerDiscoveryHandler struct {
	tracer *PathTracer
}

func (h *tracerDiscoveryHandler) OnEntityDiscovered(entity *Entity) {
	h.tracer.updateEntityCache(entity)
}

func (h *tracerDiscoveryHandler) OnEntityUpdated(entity *Entity) {
	h.tracer.updateEntityCache(entity)
}

func (h *tracerDiscoveryHandler) OnEntityDeleted(entityID string) {
	h.tracer.removeEntityFromCache(entityID)
}

func (h *tracerDiscoveryHandler) OnRelationDiscovered(relation *Relation) {}
func (h *tracerDiscoveryHandler) OnRelationDeleted(relationID string)    {}

// Start 启动追踪器
func (t *PathTracer) Start() error {
	// 启动路径清理协程
	t.wg.Add(1)
	go t.cleanupLoop()
	
	return nil
}

// Stop 停止追踪器
func (t *PathTracer) Stop() {
	t.cancel()
	t.wg.Wait()
}

// RegisterHandler 注册追踪事件处理器
func (t *PathTracer) RegisterHandler(handler TraceHandler) {
	t.handlers = append(t.handlers, handler)
}

// cleanupLoop 定期清理过期路径
func (t *PathTracer) cleanupLoop() {
	defer t.wg.Done()
	
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.cleanupExpiredPaths()
		}
	}
}

// cleanupExpiredPaths 清理过期路径
func (t *PathTracer) cleanupExpiredPaths() {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	now := time.Now()
	for id, path := range t.activePaths {
		if now.Sub(path.StartTime) > t.config.PathTimeout {
			path.Status = "timeout"
			path.EndTime = now
			path.Duration = now.Sub(path.StartTime)
			t.paths[id] = path
			delete(t.activePaths, id)
			
			// 通知完成
			for _, handler := range t.handlers {
				handler.OnPathComplete(path)
			}
		}
	}
	
	// 限制缓存大小
	if len(t.paths) > t.config.MaxPaths {
		// 删除最旧的路径
		var oldestID string
		var oldestTime time.Time
		for id, path := range t.paths {
			if oldestID == "" || path.StartTime.Before(oldestTime) {
				oldestID = id
				oldestTime = path.StartTime
			}
		}
		if oldestID != "" {
			delete(t.paths, oldestID)
		}
	}
}

// updateEntityCache 更新实体缓存
func (t *PathTracer) updateEntityCache(entity *Entity) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entityCache[entity.ID] = entity
}

// removeEntityFromCache 从缓存移除实体
func (t *PathTracer) removeEntityFromCache(entityID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entityCache, entityID)
}

// findEntityByIP 通过IP查找实体
func (t *PathTracer) findEntityByIP(ip string) *Entity {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	for _, entity := range t.entityCache {
		for _, entityIP := range entity.IPAddresses {
			if entityIP == ip {
				return entity
			}
		}
	}
	
	return nil
}

// findEntityByPort 通过端口查找实体
func (t *PathTracer) findEntityByPort(ip string, port uint16) *Entity {
	// 首先通过IP查找
	entity := t.findEntityByIP(ip)
	if entity != nil {
		return entity
	}
	
	return nil
}

// StartPath 开始追踪路径
func (t *PathTracer) StartPath(traceID string, source *TraceEndpoint, dest *TraceEndpoint, protocol string) *TracePath {
	path := &TracePath{
		ID:          generatePathID(traceID),
		TraceID:     traceID,
		StartTime:   time.Now(),
		Source:      source,
		Destination: dest,
		Protocol:    protocol,
		Status:      "in_progress",
		Hops:        make([]*TraceHop, 0),
		Labels:      make(map[string]string),
	}
	
	// 关联实体
	if source.IP != "" {
		entity := t.findEntityByIP(source.IP)
		if entity != nil {
			source.EntityID = entity.ID
			source.EntityType = entity.Type
			source.EntityName = entity.Name
			source.ContainerID = entity.ContainerID
			source.PodName = entity.PodName
			source.Namespace = entity.Namespace
			source.NodeName = entity.NodeName
		}
	}
	
	t.mu.Lock()
	t.activePaths[path.ID] = path
	t.mu.Unlock()
	
	// 通知开始
	for _, handler := range t.handlers {
		handler.OnPathStart(path)
	}
	
	return path
}

// AddHop 添加跳跃节点
func (t *PathTracer) AddHop(pathID string, hop *TraceHop) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	path, exists := t.activePaths[pathID]
	if !exists {
		return fmt.Errorf("path not found: %s", pathID)
	}
	
	// 设置跳跃序号
	hop.ID = len(path.Hops) + 1
	
	// 关联实体
	if hop.IP != "" {
		entity := t.findEntityByIP(hop.IP)
		if entity != nil {
			hop.EntityID = entity.ID
			hop.EntityType = entity.Type
			hop.EntityName = entity.Name
		}
	}
	
	path.Hops = append(path.Hops, hop)
	
	// 通知跳跃发现
	for _, handler := range t.handlers {
		handler.OnHopDiscovered(path, hop)
	}
	
	return nil
}

// CompletePath 完成追踪路径
func (t *PathTracer) CompletePath(pathID string, status string, statusCode int, bytesSent, bytesRecv uint64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	path, exists := t.activePaths[pathID]
	if !exists {
		return fmt.Errorf("path not found: %s", pathID)
	}
	
	path.EndTime = time.Now()
	path.Duration = path.EndTime.Sub(path.StartTime)
	path.Status = status
	path.StatusCode = statusCode
	path.BytesSent = bytesSent
	path.BytesRecv = bytesRecv
	
	// 移动到完成列表
	t.paths[pathID] = path
	delete(t.activePaths, pathID)
	
	// 通知完成
	for _, handler := range t.handlers {
		handler.OnPathComplete(path)
	}
	
	return nil
}

// FailPath 标记路径失败
func (t *PathTracer) FailPath(pathID string, errorCode, errorMessage string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	path, exists := t.activePaths[pathID]
	if !exists {
		return fmt.Errorf("path not found: %s", pathID)
	}
	
	path.EndTime = time.Now()
	path.Duration = path.EndTime.Sub(path.StartTime)
	path.Status = "failed"
	path.ErrorCode = errorCode
	path.ErrorMessage = errorMessage
	
	t.paths[pathID] = path
	delete(t.activePaths, pathID)
	
	for _, handler := range t.handlers {
		handler.OnPathComplete(path)
	}
	
	return nil
}

// GetPath 获取路径
func (t *PathTracer) GetPath(pathID string) *TracePath {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if path, exists := t.paths[pathID]; exists {
		return path
	}
	
	return t.activePaths[pathID]
}

// GetPathsByTraceID 通过TraceID获取路径
func (t *PathTracer) GetPathsByTraceID(traceID string) []*TracePath {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	var paths []*TracePath
	
	for _, path := range t.paths {
		if path.TraceID == traceID {
			paths = append(paths, path)
		}
	}
	
	for _, path := range t.activePaths {
		if path.TraceID == traceID {
			paths = append(paths, path)
		}
	}
	
	return paths
}

// GetActivePaths 获取活跃路径
func (t *PathTracer) GetActivePaths() []*TracePath {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	paths := make([]*TracePath, 0, len(t.activePaths))
	for _, path := range t.activePaths {
		paths = append(paths, path)
	}
	
	return paths
}

// GetCompletedPaths 获取已完成路径
func (t *PathTracer) GetCompletedPaths(limit int) []*TracePath {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	paths := make([]*TracePath, 0, len(t.paths))
	for _, path := range t.paths {
		paths = append(paths, path)
		if limit > 0 && len(paths) >= limit {
			break
		}
	}
	
	return paths
}

// TraceConnection 追踪连接
func (t *PathTracer) TraceConnection(srcIP string, srcPort uint16, dstIP string, dstPort uint16, protocol string) *TracePath {
	traceID := generateTraceID()
	
	// 创建端点
	source := &TraceEndpoint{
		IP:   srcIP,
		Port: srcPort,
	}
	
	destination := &TraceEndpoint{
		IP:   dstIP,
		Port: dstPort,
	}
	
	// 关联实体
	if entity := t.findEntityByIP(srcIP); entity != nil {
		source.EntityID = entity.ID
		source.EntityType = entity.Type
		source.EntityName = entity.Name
	}
	
	if entity := t.findEntityByIP(dstIP); entity != nil {
		destination.EntityID = entity.ID
		destination.EntityType = entity.Type
		destination.EntityName = entity.Name
	}
	
	return t.StartPath(traceID, source, destination, protocol)
}

// TraceHTTPRequest 追踪HTTP请求
func (t *PathTracer) TraceHTTPRequest(srcIP string, srcPort uint16, dstIP string, dstPort uint16, method, path string, headers map[string]string) *TracePath {
	tracePath := t.TraceConnection(srcIP, srcPort, dstIP, dstPort, "HTTP")
	tracePath.Method = method
	tracePath.Path = path
	
	// 从headers提取TraceID
	if traceID, ok := headers["X-Trace-ID"]; ok {
		tracePath.TraceID = traceID
	}
	
	return tracePath
}

// TraceTCPConnection 追踪TCP连接
func (t *PathTracer) TraceTCPConnection(srcIP string, srcPort uint16, dstIP string, dstPort uint16) *TracePath {
	return t.TraceConnection(srcIP, srcPort, dstIP, dstPort, "TCP")
}

// TraceUDPConnection 追踪UDP连接
func (t *PathTracer) TraceUDPConnection(srcIP string, srcPort uint16, dstIP string, dstPort uint16) *TracePath {
	return t.TraceConnection(srcIP, srcPort, dstIP, dstPort, "UDP")
}

// BuildTopologyGraph 构建拓扑图
func (t *PathTracer) BuildTopologyGraph() *TopologyGraph {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	graph := &TopologyGraph{
		Nodes: make(map[string]*TopologyNode),
		Edges: make(map[string]*TopologyEdge),
	}
	
	// 从路径构建节点和边
	for _, path := range t.paths {
		// 添加源节点
		if path.Source != nil && path.Source.EntityID != "" {
			node := graph.GetOrCreateNode(path.Source.EntityID, path.Source.EntityType, path.Source.EntityName)
			node.RequestCount++
			node.BytesSent += path.BytesSent
		}
		
		// 添加目标节点
		if path.Destination != nil && path.Destination.EntityID != "" {
			node := graph.GetOrCreateNode(path.Destination.EntityID, path.Destination.EntityType, path.Destination.EntityName)
			node.RequestCount++
			node.BytesRecv += path.BytesRecv
		}
		
		// 添加边
		if path.Source != nil && path.Destination != nil {
			edgeID := fmt.Sprintf("%s-%s", path.Source.EntityID, path.Destination.EntityID)
			edge := graph.GetOrCreateEdge(edgeID, path.Source.EntityID, path.Destination.EntityID)
			edge.RequestCount++
			edge.TotalLatency += path.Duration
			if path.Status == "failed" {
				edge.ErrorCount++
			}
		}
		
		// 添加跳跃节点
		for _, hop := range path.Hops {
			if hop.EntityID != "" {
				node := graph.GetOrCreateNode(hop.EntityID, hop.EntityType, hop.EntityName)
				node.HopCount++
			}
		}
	}
	
	// 计算平均延迟
	for _, edge := range graph.Edges {
		if edge.RequestCount > 0 {
			edge.AvgLatency = edge.TotalLatency / time.Duration(edge.RequestCount)
		}
	}
	
	return graph
}

// TopologyGraph 拓扑图
type TopologyGraph struct {
	Nodes map[string]*TopologyNode `json:"nodes"`
	Edges map[string]*TopologyEdge `json:"edges"`
}

// TopologyNode 拓扑节点
type TopologyNode struct {
	ID           string     `json:"id"`
	Type         EntityType `json:"type"`
	Name         string     `json:"name"`
	RequestCount int64      `json:"request_count"`
	BytesSent    uint64     `json:"bytes_sent"`
	BytesRecv    uint64     `json:"bytes_recv"`
	HopCount     int64      `json:"hop_count"`
}

// TopologyEdge 拓扑边
type TopologyEdge struct {
	ID           string        `json:"id"`
	SourceID     string        `json:"source_id"`
	TargetID     string        `json:"target_id"`
	RequestCount int64         `json:"request_count"`
	ErrorCount   int64         `json:"error_count"`
	TotalLatency time.Duration `json:"total_latency"`
	AvgLatency   time.Duration `json:"avg_latency"`
}

// GetOrCreateNode 获取或创建节点
func (g *TopologyGraph) GetOrCreateNode(id string, entityType EntityType, name string) *TopologyNode {
	if node, exists := g.Nodes[id]; exists {
		return node
	}
	
	node := &TopologyNode{
		ID:   id,
		Type: entityType,
		Name: name,
	}
	g.Nodes[id] = node
	return node
}

// GetOrCreateEdge 获取或创建边
func (g *TopologyGraph) GetOrCreateEdge(id, sourceID, targetID string) *TopologyEdge {
	if edge, exists := g.Edges[id]; exists {
		return edge
	}
	
	edge := &TopologyEdge{
		ID:       id,
		SourceID: sourceID,
		TargetID: targetID,
	}
	g.Edges[id] = edge
	return edge
}

// ToJSON 转换为JSON
func (g *TopologyGraph) ToJSON() ([]byte, error) {
	return json.Marshal(g)
}

// PathStats 路径统计
type PathStats struct {
	TotalPaths     int64         `json:"total_paths"`
	ActivePaths    int64         `json:"active_paths"`
	SuccessPaths   int64         `json:"success_paths"`
	FailedPaths    int64         `json:"failed_paths"`
	TimeoutPaths   int64         `json:"timeout_paths"`
	AvgDuration    time.Duration `json:"avg_duration"`
	AvgHops        float64       `json:"avg_hops"`
	AvgLatency     time.Duration `json:"avg_latency"`
	TotalBytesSent uint64        `json:"total_bytes_sent"`
	TotalBytesRecv uint64        `json:"total_bytes_recv"`
}

// GetStats 获取路径统计
func (t *PathTracer) GetStats() *PathStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	stats := &PathStats{
		ActivePaths: int64(len(t.activePaths)),
	}
	
	var totalDuration time.Duration
	var totalHops int
	var totalLatency time.Duration
	
	for _, path := range t.paths {
		stats.TotalPaths++
		totalDuration += path.Duration
		totalHops += len(path.Hops)
		stats.TotalBytesSent += path.BytesSent
		stats.TotalBytesRecv += path.BytesRecv
		
		switch path.Status {
		case "success":
			stats.SuccessPaths++
		case "failed":
			stats.FailedPaths++
		case "timeout":
			stats.TimeoutPaths++
		}
		
		// 计算延迟
		for _, hop := range path.Hops {
			totalLatency += hop.Latency
		}
	}
	
	if stats.TotalPaths > 0 {
		stats.AvgDuration = totalDuration / time.Duration(stats.TotalPaths)
		stats.AvgHops = float64(totalHops) / float64(stats.TotalPaths)
	}
	
	return stats
}

// generatePathID 生成路径ID
func generatePathID(traceID string) string {
	return fmt.Sprintf("path-%s-%d", traceID[:8], time.Now().UnixNano())
}

// generateTraceID 生成TraceID
func generateTraceID() string {
	return fmt.Sprintf("%016x", time.Now().UnixNano())
}

// ResolveEntity 解析实体信息
func (t *PathTracer) ResolveEntity(ip string, port uint16) *TraceEndpoint {
	endpoint := &TraceEndpoint{
		IP:   ip,
		Port: port,
	}
	
	// 查找实体
	entity := t.findEntityByIP(ip)
	if entity != nil {
		endpoint.EntityID = entity.ID
		endpoint.EntityType = entity.Type
		endpoint.EntityName = entity.Name
		endpoint.ContainerID = entity.ContainerID
		endpoint.PodName = entity.PodName
		endpoint.Namespace = entity.Namespace
		endpoint.NodeName = entity.NodeName
	}
	
	// DNS反向解析
	if t.config.EnableDNS {
		names, _ := net.LookupAddr(ip)
		if len(names) > 0 {
			endpoint.ServiceName = names[0]
		}
	}
	
	return endpoint
}

// FindPathBetween 查找两个实体之间的路径
func (t *PathTracer) FindPathBetween(sourceID, targetID string) []*TracePath {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	var paths []*TracePath
	
	for _, path := range t.paths {
		if path.Source != nil && path.Destination != nil {
			if path.Source.EntityID == sourceID && path.Destination.EntityID == targetID {
				paths = append(paths, path)
			}
		}
	}
	
	return paths
}

// FindPathsByEntity 查找涉及某实体的所有路径
func (t *PathTracer) FindPathsByEntity(entityID string) []*TracePath {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	var paths []*TracePath
	
	for _, path := range t.paths {
		// 检查源和目标
		if path.Source != nil && path.Source.EntityID == entityID {
			paths = append(paths, path)
			continue
		}
		if path.Destination != nil && path.Destination.EntityID == entityID {
			paths = append(paths, path)
			continue
		}
		
		// 检查跳跃节点
		for _, hop := range path.Hops {
			if hop.EntityID == entityID {
				paths = append(paths, path)
				break
			}
		}
	}
	
	return paths
}

// GetEntityDependencies 获取实体依赖关系
func (t *PathTracer) GetEntityDependencies(entityID string) map[string]int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	deps := make(map[string]int64)
	
	for _, path := range t.paths {
		if path.Source != nil && path.Source.EntityID == entityID {
			if path.Destination != nil && path.Destination.EntityID != "" {
				deps[path.Destination.EntityID]++
			}
		}
	}
	
	return deps
}

// GetEntityDependents 获取依赖某实体的实体列表
func (t *PathTracer) GetEntityDependents(entityID string) map[string]int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	deps := make(map[string]int64)
	
	for _, path := range t.paths {
		if path.Destination != nil && path.Destination.EntityID == entityID {
			if path.Source != nil && path.Source.EntityID != "" {
				deps[path.Source.EntityID]++
			}
		}
	}
	
	return deps
}
