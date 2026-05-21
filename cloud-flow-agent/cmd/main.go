package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"cloud-flow-agent/internal/collector"
	"cloud-flow-agent/internal/config"
	"cloud-flow-agent/internal/ebpfcollector"
	"cloud-flow-agent/internal/grpcclient"
	"cloud-flow-agent/internal/http"
	"cloud-flow-agent/pkg/logger"
	"cloud-flow-agent/pkg/metrics"
	edge "cloud-flow/proto"
)

// safeClient 线程安全的客户端包装器
type safeClient struct {
	mu     sync.RWMutex
	client *grpcclient.Client
}

// Get 安全地获取客户端
func (sc *safeClient) Get() *grpcclient.Client {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.client
}

// Set 安全地设置客户端
func (sc *safeClient) Set(c *grpcclient.Client) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.client = c
}

// Version 版本号，可在编译时通过 -ldflags "-X main.Version=x.y.z" 注入
var Version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(logger.Config{Level: cfg.Log.Level, Format: cfg.Log.Format})
	defer log.Sync()

	// 初始化指标收集器
	metricCollector := metrics.New()

	// 启动 Prometheus 指标服务器
	metricsAddr := fmt.Sprintf(":%s", cfg.MetricsPort)
	metricsServer, metricsErrCh := metricCollector.StartServer(metricsAddr)
	go func() {
		if err := <-metricsErrCh; err != nil {
			log.Warnf("启动 Prometheus 指标服务器失败: %v", err)
		}
	}()

	log.Infof("探针启动中... 配置: %s", cfg.Summary())

	// 连接边缘节点（带重试，Edge 未启动时自动等待）
	var client *grpcclient.Client
	connectDelay := 2 * time.Second
	maxRetries := cfg.MaxRetries

	for attempt := 1; ; attempt++ {
		if maxRetries > 0 && attempt > maxRetries {
			log.Errorf("连接边缘节点失败: 已达到最大重试次数 %d", maxRetries)
			return
		}

		client, err = grpcclient.NewClient(cfg.EdgeAddr, cfg.APIKey, grpcclient.TLSConfig{
			Enabled:    cfg.TLS.Enabled,
			ServerName: cfg.TLS.ServerName,
			CACert:     cfg.TLS.CACert,
			ClientCert: cfg.TLS.ClientCert,
			ClientKey:  cfg.TLS.ClientKey,
		}, log)
		if err == nil {
			break
		}
		log.Warnf("连接边缘节点失败 (第 %d 次): %v，%s 后重试...", attempt, err, connectDelay)
		time.Sleep(connectDelay)
		if connectDelay < 30*time.Second {
			connectDelay *= 2
		}
	}

	// 创建线程安全的客户端包装器
	safeClient := &safeClient{client: client}

	// 注册探针
	hostname, _ := os.Hostname()
	hostIP := getLocalIP()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := safeClient.Get().Register(ctx, cfg.ProbeID, hostIP, hostname, Version)
	if err != nil {
		log.Errorf("注册探针失败: %v", err)
		safeClient.Get().Close()
		os.Exit(1)
	}
	log.Infof("注册成功: %s, 心跳间隔=%ds", resp.GetMessage(), resp.GetHeartbeatInterval())

	// 初始化传统采集器
	c := collector.New(collector.CollectConfig{
		CPU:     cfg.Collect.CPU,
		Memory:  cfg.Collect.Memory,
		Network: cfg.Collect.Network,
		Disk:    cfg.Collect.Disk,
	})

	// 初始化 EBPF 采集器（如果可用）
	var ebpfCollector *ebpfcollector.Collector
	if cfg.EBPF.Enabled {
		ebpfOpts := &ebpfcollector.CollectorOptions{
			EnableTCPMetrics:  cfg.EBPF.TCPMetrics.Enabled,
			EnableHTTPMetrics: cfg.EBPF.HTTPMetrics.Enabled,
			EnableHTTPFull:    cfg.EBPF.ProtocolParsing.Enabled && cfg.EBPF.ProtocolParsing.HTTPFull,
			EnableDNSFull:     cfg.EBPF.ProtocolParsing.Enabled && cfg.EBPF.ProtocolParsing.DNSFull,
			EnableMySQLFull:   cfg.EBPF.ProtocolParsing.Enabled && cfg.EBPF.ProtocolParsing.MySQLFull,
			MgmtIface:         cfg.Network.MgmtIface,
		}

		ebpfCollector, err = ebpfcollector.NewWithOptions(ebpfOpts)
		if err != nil {
			log.Warnf("EBPF 采集器初始化失败: %v，将只使用传统采集器", err)
		} else {
			log.Info("EBPF 采集器初始化成功，开始采集网络流量")
			if ebpfCollector.IsTCPMetricsAvailable() {
				log.Info("TCP深度指标采集已启用: 建连时延、重传率、零窗口、队列溢出、连接失败")
			}
			if ebpfCollector.IsHTTPMetricsAvailable() {
				log.Info("HTTP应用层指标采集已启用: 请求成功率、响应时延、异常比例、请求数、响应数")
			}
			if ebpfCollector.IsHTTPFullAvailable() {
				log.Info("HTTP全字段解析已启用: 方法、路径、Host、Cookie、User-Agent、状态码等")
			}
			if ebpfCollector.IsDNSFullAvailable() {
				log.Info("DNS全字段解析已启用: 域名、记录类型、TTL、响应码等")
			}
			if ebpfCollector.IsMySQLFullAvailable() {
				log.Info("MySQL全字段解析已启用: 命令、SQL语句、错误码、影响行数等")
			}
			ebpfCollector.Start()
			defer ebpfCollector.Stop()
		}
	} else {
		log.Info("EBPF 采集已禁用，使用传统采集器")
	}

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	var stopOnce sync.Once

	// 心跳协程
	heartbeatInterval := time.Duration(resp.GetHeartbeatInterval()) * time.Second
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		failureCount := 0
		const maxFailures = 3 // 连续失败3次触发重连
		const reconnectDelay = 5 * time.Second

		for {
			select {
			case <-ticker.C:
				heartbeatCtx, heartbeatCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := safeClient.Get().Heartbeat(heartbeatCtx, cfg.ProbeID); err != nil {
					failureCount++
					log.Warnf("心跳失败 (第 %d/%d 次): %v", failureCount, maxFailures, err)
					metricCollector.HeartbeatSent(err)

					// 连续失败达到阈值，触发重连
					if failureCount >= maxFailures {
						log.Warnf("连续心跳失败 %d 次，开始重连 Edge 节点...", maxFailures)

						// 关闭当前客户端
						safeClient.Get().Close()

						// 重新连接 Edge 节点
						var newClient *grpcclient.Client
						connectDelay := 2 * time.Second
						for attempt := 1; ; attempt++ {
							newClient, err = grpcclient.NewClient(cfg.EdgeAddr, cfg.APIKey, grpcclient.TLSConfig{
								Enabled:    cfg.TLS.Enabled,
								ServerName: cfg.TLS.ServerName,
								CACert:     cfg.TLS.CACert,
								ClientCert: cfg.TLS.ClientCert,
								ClientKey:  cfg.TLS.ClientKey,
							}, log)
							if err == nil {
								break
							}
							log.Warnf("重连 Edge 节点失败 (第 %d 次): %v，%s 后重试...", attempt, err, connectDelay)
							select {
							case <-stopCh:
								return
							case <-time.After(connectDelay):
							}
							if connectDelay < 30*time.Second {
								connectDelay *= 2
							}
						}

						// 重新注册
						hostname, _ = os.Hostname()
						agentIP := getLocalIP()
						regCtx, regCancel := context.WithTimeout(context.Background(), 10*time.Second)
						newResp, err := newClient.Register(regCtx, cfg.ProbeID, agentIP, hostname, Version)
						regCancel()
						if err != nil {
							log.Errorf("重新注册失败: %v", err)
							failureCount = 0
							continue
						}
						// 检查注册是否成功
						if !newResp.GetSuccess() {
							log.Errorf("重新注册被拒绝: %s，请检查 API Key 配置", newResp.GetMessage())
							failureCount = 0
							// API Key 不匹配时停止重试，避免无限循环
							log.Errorf("API Key 配置错误，探针将停止运行，请修改配置后重启")
							stopOnce.Do(func() {
								close(stopCh)
							})
							return
						}

						// 重连成功，更新客户端和心跳间隔
						safeClient.Set(newClient)
						heartbeatInterval = time.Duration(newResp.GetHeartbeatInterval()) * time.Second
						ticker.Reset(heartbeatInterval)
						failureCount = 0
						log.Infof("重连成功: %s, 心跳间隔=%ds", newResp.GetMessage(), newResp.GetHeartbeatInterval())
					}
				} else {
					// 心跳成功，重置失败计数
					failureCount = 0
					metricCollector.HeartbeatSent(nil)
				}
				heartbeatCancel() // 直接调用而非 defer，因为 defer 在 for 循环内会延迟到函数退出才执行
			case <-stopCh:
				return
			}
		}
	}()

	// 指标采集 + 发送协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Duration(cfg.CollectInterval) * time.Second)
		defer ticker.Stop()
		flushTicker := time.NewTicker(30 * time.Second) // 每30秒flush一次日志
		defer flushTicker.Stop()
		var buf []*edge.MetricData

		for {
			select {
			case <-ticker.C:
				// 采集传统指标
				start := metricCollector.CollectStarted()
				metricsData, collectErr := c.Collect()
				metricCollector.CollectFinished(start, collectErr)
				if collectErr != nil {
					log.Warnf("采集传统指标失败: %v", collectErr)
				} else {
					buf = append(buf, metricsData...)
				}

				// 采集 EBPF 网络流量数据
				if ebpfCollector != nil {
					ebpfMetrics := ebpfCollector.Collect()
					metricCollector.EBPFCollect(nil)
					buf = append(buf, ebpfMetrics...)
				}

				if len(buf) >= cfg.BatchSize {
					batch := &edge.MetricsBatch{
						ProbeId: cfg.ProbeID,
						Metrics: buf,
					}

					// 发送指标
					sendStart := metricCollector.SendStarted()
					sendCtx, sendCancel := context.WithTimeout(context.Background(), 10*time.Second)
					err := safeClient.Get().SendMetrics(sendCtx, batch)
					sendCancel() // 统一在 SendMetrics 返回后立即调用
					if err != nil {
						log.Errorf("发送指标数据失败: %v", err)
						metricCollector.SendFinished(sendStart, 0, err)
						// 保留数据下次重试（简单丢弃超出阈值的部分避免内存溢出）
						// 保留最新的 cfg.BatchSize 条数据，因为最新数据通常更有价值
						if len(buf) > cfg.BatchSize*2 {
							buf = buf[len(buf)-cfg.BatchSize:]
						}
					} else {
						log.Debugf("发送 %d 条指标数据", len(buf))
						sentLen := len(buf)
						metricCollector.SendFinished(sendStart, sentLen*100, nil) // 估算字节数
						buf = nil
					}
				}
			case <-flushTicker.C:
				// 每30秒flush一次日志
				log.Sync()
			case <-stopCh:
				// 发送剩余数据
				if len(buf) > 0 {
					batch := &edge.MetricsBatch{
						ProbeId: cfg.ProbeID,
						Metrics: buf,
					}
					sendStart := metricCollector.SendStarted()
					sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					if err := safeClient.Get().SendMetrics(sendCtx, batch); err != nil {
						log.Warnf("发送剩余数据失败: %v", err)
						metricCollector.SendFinished(sendStart, 0, err)
					} else {
						metricCollector.SendFinished(sendStart, len(buf)*100, nil) // 估算字节数
					}
					cancel()
				}
				return
			}
		}
	}()

	// 启动 HTTP 健康检查服务器
	http.Version = Version
	healthHandler := http.NewHealthHandler(safeClient, c, ebpfCollector, nil, log)
	healthAddr := fmt.Sprintf(":%s", cfg.HealthPort)
	healthServer := http.StartHealthServer(healthAddr, healthHandler)
	log.Infof("健康检查 HTTP 服务监听: %s/health", healthAddr)

	// 初始化 ON-CPU 剖析器（如果启用）
	if cfg.EBPF.CPUProfiler.Enabled {
		log.Infof("ON-CPU 剖析器已启用: 采样频率=%dHz, 目标PID=%d, 输出目录=%s",
			cfg.EBPF.CPUProfiler.SampleFreq, cfg.EBPF.CPUProfiler.TargetPID, cfg.EBPF.CPUProfiler.OutputDir)
	}

	log.Info("探针已启动")

	// 信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Infof("收到信号 %v，正在退出...", sig)

	// 优雅退出超时
	const gracefulShutdownTimeout = 30 * time.Second
	ctx, cancel = context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	// 停止所有goroutine
	stopOnce.Do(func() {
		close(stopCh)
	})

	// 等待goroutine退出
	done := make(chan struct{})
	go func() {
		// 等待所有goroutine完成
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("所有goroutine已安全退出")
	case <-ctx.Done():
		log.Warnf("优雅退出超时，强制关闭")
	}

	// 关闭 Prometheus 指标服务器
	if metricsServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			log.Warnf("关闭 Prometheus 指标服务器失败: %v", err)
		} else {
			log.Info("Prometheus 指标服务器已优雅关闭")
		}
	}

	// 关闭 HTTP 健康检查服务器
	if healthServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := healthServer.Shutdown(shutdownCtx); err != nil {
			log.Warnf("关闭 HTTP 服务器失败: %v", err)
		} else {
			log.Info("HTTP 服务器已优雅关闭")
		}
	}

	// 关闭客户端连接
	safeClient.Get().Close()
	log.Info("探针已安全退出")
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		hostname, _ := os.Hostname()
		if hostname != "" {
			return hostname
		}
		return "0.0.0.0" // 最终兜底值
	}

	// 优先返回 IPv4 地址
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	// 如果没有 IPv4 地址，返回 IPv6 地址
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			return ipnet.IP.String()
		}
	}

	hostname, _ := os.Hostname()
	if hostname != "" {
		return hostname
	}
	return "0.0.0.0" // 最终兜底值
}
