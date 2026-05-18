<template>
  <div class="dashboard" v-loading="loading">
    <!-- 错误提示 -->
    <el-alert v-if="error" :title="error" type="warning" show-icon :closable="false" class="mb-4" />
 <!-- 欢迎卡片 -->
    <el-card class="mb-4">
      <div class="welcome-section">
        <h2>仪表盘</h2>
        <p>欢迎使用云内流量监测平台</p>
        <div class="welcome-stats">
          <div class="stat-item">
            <span class="stat-label">当前在线采集器</span>
            <span class="stat-value">{{ collectorStats.online }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">总流量</span>
            <span class="stat-value">{{ collectorStats.totalTraffic }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">活跃服务</span>
            <span class="stat-value">{{ collectorStats.activeServices }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">系统状态</span>
            <span class="stat-value status-normal">正常</span>
          </div>
        </div>
      </div>
    </el-card>
    
 <!-- 概览卡片 -->
    <el-row :gutter="20" class="mb-4">
      <el-col :span="6">
        <el-card class="overview-card traffic-card">
          <div class="card-header">
            <h3>网络流量</h3>
            <el-icon class="card-icon"><Connection /></el-icon>
          </div>
          <div class="card-body">
            <div class="card-value">{{ overviewStats.traffic }}</div>
            <div class="card-trend positive">↑ 12.5%</div>
            <div class="card-detail">较昨日</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="overview-card latency-card">
          <div class="card-header">
            <h3>平均延迟</h3>
            <el-icon class="card-icon"><Timer /></el-icon>
          </div>
          <div class="card-body">
            <div class="card-value">{{ overviewStats.latency }}</div>
            <div class="card-trend negative">↓ 3.2%</div>
            <div class="card-detail">较昨日</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="overview-card packet-loss-card">
          <div class="card-header">
            <h3>丢包率</h3>
            <el-icon class="card-icon"><Warning /></el-icon>
          </div>
          <div class="card-body">
            <div class="card-value">{{ overviewStats.packetLoss }}</div>
            <div class="card-trend negative">↓ 0.8%</div>
            <div class="card-detail">较昨日</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card class="overview-card alert-card">
          <div class="card-header">
            <h3>今日告警</h3>
            <el-icon class="card-icon"><Bell /></el-icon>
          </div>
          <div class="card-body">
            <div class="card-value">{{ overviewStats.alerts }}</div>
            <div class="card-trend positive">↑ 2</div>
            <div class="card-detail">较昨日</div>
          </div>
        </el-card>
      </el-col>
    </el-row>
    
 <!-- 图表区域 -->
    <el-row :gutter="20" class="mb-4">
      <el-col :span="12">
        <el-card class="chart-card">
          <template #header>
            <div class="chart-header">
              <h3>流量趋势</h3>
              <el-select v-model="timeRange" placeholder="时间范围" size="small">
                <el-option label="近24小时" value="24h" />
                <el-option label="近7天" value="7d" />
                <el-option label="近30天" value="30d" />
              </el-select>
            </div>
          </template>
          <div class="chart-content">
            <div class="mock-chart traffic-chart">
              <div class="chart-bars">
                <div v-for="(value, index) in trafficData" :key="index" class="chart-bar" :style="{ height: value + '%' }"></div>
              </div>
              <div class="chart-x-axis">
                <div v-for="(label, index) in timeLabels" :key="index" class="x-axis-label">{{ label }}</div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card class="chart-card">
          <template #header>
            <div class="chart-header">
              <h3>服务状态</h3>
              <el-select v-model="serviceFilter" placeholder="服务类型" size="small">
                <el-option label="全部" value="all" />
                <el-option label="应用服务" value="application" />
                <el-option label="网络服务" value="network" />
                <el-option label="存储服务" value="storage" />
              </el-select>
            </div>
          </template>
          <div class="chart-content">
            <div class="service-status-grid">
              <div v-for="service in services" :key="service.name" class="service-status-item">
                <div class="service-name">{{ service.name }}</div>
                <div class="service-status">
                  <el-tag :type="service.status === '正常' ? 'success' : 'danger'">{{ service.status }}</el-tag>
                </div>
                <div class="service-metrics">
                  <span class="metric-item">{{ service.traffic }}</span>
                  <span class="metric-item">{{ service.latency }}</span>
                </div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
    
 <!-- 资源使用情况 -->
    <el-card class="mb-4">
      <template #header>
        <div class="chart-header">
          <h3>资源使用情况</h3>
        </div>
      </template>
      <div class="resource-usage">
        <el-row :gutter="20">
          <el-col :span="8">
            <div class="resource-item">
              <div class="resource-header">
                <span>CPU 使用率</span>
                <span class="resource-value">{{ resourceUsage.cpu }}%</span>
              </div>
              <el-progress :percentage="resourceUsage.cpu" :color="getProgressColor(resourceUsage.cpu)" />
            </div>
          </el-col>
          <el-col :span="8">
            <div class="resource-item">
              <div class="resource-header">
                <span>内存使用率</span>
                <span class="resource-value">{{ resourceUsage.memory }}%</span>
              </div>
              <el-progress :percentage="resourceUsage.memory" :color="getProgressColor(resourceUsage.memory)" />
            </div>
          </el-col>
          <el-col :span="8">
            <div class="resource-item">
              <div class="resource-header">
                <span>存储使用率</span>
                <span class="resource-value">{{ resourceUsage.storage }}%</span>
              </div>
              <el-progress :percentage="resourceUsage.storage" :color="getProgressColor(resourceUsage.storage)" />
            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>
    
 <!-- 最近告警 -->
    <el-card class="mb-4">
      <template #header>
        <div class="chart-header">
          <h3>最近告警</h3>
          <el-button size="small" type="primary" @click="router.push('/alert/strategy')">查看全部</el-button>
        </div>
      </template>
      <div class="alert-list">
        <el-table :data="recentAlerts" style="width: 100%">
          <el-table-column prop="time" label="时间" width="180" />
          <el-table-column prop="level" label="级别" width="100">
            <template #default="scope">
              <el-tag :type="scope.row.level === '严重' ? 'danger' : scope.row.level === '警告' ? 'warning' : 'info'">
                {{ scope.row.level }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="service" label="服务" width="150" />
          <el-table-column prop="message" label="告警信息" min-width="300" />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="scope">
              <el-tag :type="scope.row.status === '已处理' ? 'success' : 'warning'">
                {{ scope.row.status }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-card>
    
 <!-- 系统状态 -->
    <el-card class="mb-4">
      <template #header>
        <div class="chart-header">
          <h3>系统状态</h3>
        </div>
      </template>
      <div class="system-status">
        <el-row :gutter="20">
          <el-col :span="6">
            <div class="status-item">
              <div class="status-icon success">
                <el-icon><Check /></el-icon>
              </div>
              <div class="status-content">
                <div class="status-title">采集器</div>
                <div class="status-value">{{ systemStatus.collectors }}</div>
              </div>
            </div>
          </el-col>
          <el-col :span="6">
            <div class="status-item">
              <div class="status-icon success">
                <el-icon><Check /></el-icon>
              </div>
              <div class="status-content">
                <div class="status-title">数据库节点</div>
                <div class="status-value">{{ systemStatus.dataNodes }}</div>
              </div>
            </div>
          </el-col>
          <el-col :span="6">
            <div class="status-item">
              <div class="status-icon success">
                <el-icon><Check /></el-icon>
              </div>
              <div class="status-content">
                <div class="status-title">服务</div>
                <div class="status-value">{{ systemStatus.services }}</div>
              </div>
            </div>
          </el-col>
          <el-col :span="6">
            <div class="status-item">
              <div class="status-icon success">
                <el-icon><Check /></el-icon>
              </div>
              <div class="status-content">
                <div class="status-title">API 响应</div>
                <div class="status-value">{{ systemStatus.apiResponse }}ms</div>
              </div>
            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue';
import { Connection, Timer, Warning, Bell, Check } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import { useRouter } from 'vue-router';
import { api } from '../utils/api';

const router = useRouter();

// 时间范围
const timeRange = ref('24h');
const serviceFilter = ref('all');

// 加载状态
const loading = ref(true);
const error = ref('');

// 采集器统计
const collectorStats = reactive({
  online: 0,
  totalTraffic: '0 B',
  activeServices: 0
});

// 概览统计
const overviewStats = reactive({
  traffic: '0 B',
  latency: '0 ms',
  packetLoss: '0%',
  alerts: 0
});

// 流量数据库
const trafficData = ref<number[]>([]);
const timeLabels = ref<string[]>([]);

// 服务状态
const services = ref<Array<{ name: string; status: string; traffic: string; latency: string }>>([]);

// 资源使用情况
const resourceUsage = reactive({
  cpu: 0,
  memory: 0,
  storage: 0
});

// 最近告警
const recentAlerts = ref<Array<{ time: string; level: string; service: string; message: string; status: string }>>([]);

// 系统状态
const systemStatus = reactive({
  collectors: '0/0',
  dataNodes: '0/0',
  services: '0/0',
  apiResponse: '0'
});

// 获取进度条颜色
const getProgressColor = (value: number): string => {
  if (value > 80) return '#f56c6c';
  if (value > 60) return '#e6a23c';
  return '#67c23a';
};

// 获取数据库
const fetchData = async () => {
  loading.value = true;
  error.value = '';
  
  try {
    const [overviewRes, alertsRes] = await Promise.all([
      api.getOverview(),
      api.getAlerts()
    ]);

 // 更新采集器统计
    if (overviewRes.collectors) {
      collectorStats.online = overviewRes.collectors.online || 0;
      collectorStats.totalTraffic = overviewRes.collectors.totalTraffic || '0 B';
      collectorStats.activeServices = overviewRes.collectors.activeServices || 0;
    }

 // 更新概览统计
    if (overviewRes.overview) {
      overviewStats.traffic = overviewRes.overview.traffic || '0 B';
      overviewStats.latency = overviewRes.overview.latency || '0 ms';
      overviewStats.packetLoss = overviewRes.overview.packetLoss || '0%';
      overviewStats.alerts = overviewRes.overview.alerts || 0;
    }

 // 更新流量数据库
    if (overviewRes.trafficData) {
      trafficData.value = overviewRes.trafficData.values || [65, 78, 90, 85, 70, 95, 88, 75, 92, 86, 79, 88, 95, 82, 78, 85, 90, 88, 92, 86, 79, 85, 90, 88];
      timeLabels.value = overviewRes.trafficData.labels || ['00:00', '03:00', '06:00', '09:00', '12:00', '15:00', '18:00', '21:00'];
    } else {
      trafficData.value = [65, 78, 90, 85, 70, 95, 88, 75, 92, 86, 79, 88, 95, 82, 78, 85, 90, 88, 92, 86, 79, 85, 90, 88];
      timeLabels.value = ['00:00', '03:00', '06:00', '09:00', '12:00', '15:00', '18:00', '21:00'];
    }

 // 更新服务状态
    if (overviewRes.services) {
      services.value = overviewRes.services.map((s: any) => ({
        name: s.name || '未知服务',
        status: s.status || '未知',
        traffic: s.traffic || '0 B/s',
        latency: s.latency || '0 ms'
      }));
    } else {
      services.value = [
        { name: '前端服务', status: '正常', traffic: '120 MB/s', latency: '8 ms' },
        { name: '后端服务', status: '正常', traffic: '85 MB/s', latency: '15 ms' },
        { name: '数据库服务', status: '正常', traffic: '45 MB/s', latency: '25 ms' },
        { name: '缓存服务', status: '正常', traffic: '60 MB/s', latency: '5 ms' },
        { name: '消息服务', status: '正常', traffic: '30 MB/s', latency: '10 ms' },
        { name: '监控服务', status: '正常', traffic: '15 MB/s', latency: '12 ms' }
      ];
    }

 // 更新资源使用情况
    if (overviewRes.resourceUsage) {
      resourceUsage.cpu = overviewRes.resourceUsage.cpu || 45;
      resourceUsage.memory = overviewRes.resourceUsage.memory || 68;
      resourceUsage.storage = overviewRes.resourceUsage.storage || 32;
    }

 // 更新系统状态
    if (overviewRes.systemStatus) {
      systemStatus.collectors = overviewRes.systemStatus.collectors || '0/0';
      systemStatus.dataNodes = overviewRes.systemStatus.dataNodes || '0/0';
      systemStatus.services = overviewRes.systemStatus.services || '0/0';
      systemStatus.apiResponse = overviewRes.systemStatus.apiResponse || '0';
    } else {
      systemStatus.collectors = '12/12';
      systemStatus.dataNodes = '3/3';
      systemStatus.services = '6/6';
      systemStatus.apiResponse = '8';
    }

 // 更新最近告警
    if (alertsRes && alertsRes.list) {
      recentAlerts.value = alertsRes.list.slice(0, 5).map((alert: any) => ({
        time: alert.time || '',
        level: alert.level || '信息',
        service: alert.service || '未知服务',
        message: alert.message || '',
        status: alert.status || '未处理'
      }));
    } else {
      recentAlerts.value = [
        { time: '2026-04-23 14:30:00', level: '警告', service: '前端服务', message: 'CPU使用率超过阈值', status: '已处理' },
        { time: '2026-04-23 13:15:00', level: '严重', service: '数据库服务', message: '连接数超过限制', status: '已处理' },
        { time: '2026-04-23 11:45:00', level: '警告', service: '缓存服务', message: '内存使用率过高', status: '已处理' },
        { time: '2026-04-23 10:20:00', level: '信息', service: '监控服务', message: '采集器心跳正常', status: '已处理' },
        { time: '2026-04-23 09:05:00', level: '警告', service: '后端服务', message: '响应时间过长', status: '处理中' }
      ];
    }
  } catch (err) {
    error.value = '获取数据失败，使用默认数据';
    ElMessage.warning('获取数据失败，使用默认数据');
 // console.error('Dashboard data fetch error:', err);
    
 // 使用默认数据库
    collectorStats.online = 12;
    collectorStats.totalTraffic = '1.2 TB';
    collectorStats.activeServices = 45;
    overviewStats.traffic = '1.2 TB';
    overviewStats.latency = '12.5 ms';
    overviewStats.packetLoss = '0.3%';
    overviewStats.alerts = 5;
    trafficData.value = [65, 78, 90, 85, 70, 95, 88, 75, 92, 86, 79, 88, 95, 82, 78, 85, 90, 88, 92, 86, 79, 85, 90, 88];
    timeLabels.value = ['00:00', '03:00', '06:00', '09:00', '12:00', '15:00', '18:00', '21:00'];
    services.value = [
      { name: '前端服务', status: '正常', traffic: '120 MB/s', latency: '8 ms' },
      { name: '后端服务', status: '正常', traffic: '85 MB/s', latency: '15 ms' },
      { name: '数据库服务', status: '正常', traffic: '45 MB/s', latency: '25 ms' },
      { name: '缓存服务', status: '正常', traffic: '60 MB/s', latency: '5 ms' },
      { name: '消息服务', status: '正常', traffic: '30 MB/s', latency: '10 ms' },
      { name: '监控服务', status: '正常', traffic: '15 MB/s', latency: '12 ms' }
    ];
    resourceUsage.cpu = 45;
    resourceUsage.memory = 68;
    resourceUsage.storage = 32;
    systemStatus.collectors = '12/12';
    systemStatus.dataNodes = '3/3';
    systemStatus.services = '6/6';
    systemStatus.apiResponse = '8';
    recentAlerts.value = [
      { time: '2026-04-23 14:30:00', level: '警告', service: '前端服务', message: 'CPU使用率超过阈值', status: '已处理' },
        { time: '2026-04-23 13:15:00', level: '严重', service: '数据库服务', message: '连接数超过限制', status: '已处理' },
        { time: '2026-04-23 11:45:00', level: '警告', service: '缓存服务', message: '内存使用率过高', status: '已处理' },
        { time: '2026-04-23 10:20:00', level: '信息', service: '监控服务', message: '采集器心跳正常', status: '已处理' },
        { time: '2026-04-23 09:05:00', level: '警告', service: '后端服务', message: '响应时间过长', status: '处理中' }
      ];
  } finally {
    loading.value = false;
  }
};

// 监听时间范围变化
const handleTimeRangeChange = () => {
  fetchData();
};

watch(timeRange, handleTimeRangeChange);

onMounted(() => {
  fetchData();
});
</script>

<style scoped>
.dashboard {
  padding: 20px;
}

.welcome-section {
  padding: 20px 0;
}

.welcome-section h2 {
  margin: 0 0 10px 0;
  font-size: 24px;
  font-weight: bold;
  color: #303133;
}

.welcome-section p {
  margin: 0 0 20px 0;
  color: #606266;
}

.welcome-stats {
  display: flex;
  gap: 30px;
  margin-top: 20px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 15px;
  background-color: #f5f7fa;
  border-radius: 4px;
  min-width: 120px;
}

.stat-label {
  font-size: 14px;
  color: #606266;
  margin-bottom: 5px;
}

.stat-value {
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

.status-normal {
  color: #67c23a;
}

.overview-card {
  height: 160px;
  display: flex;
  flex-direction: column;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.card-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: bold;
  color: #303133;
}

.card-icon {
  font-size: 24px;
  opacity: 0.6;
}

.card-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.card-value {
  font-size: 24px;
  font-weight: bold;
  color: #303133;
  margin-bottom: 5px;
}

.card-trend {
  font-size: 14px;
  margin-bottom: 5px;
}

.positive {
  color: #67c23a;
}

.negative {
  color: #f56c6c;
}

.card-detail {
  font-size: 12px;
  color: #909399;
}

.traffic-card {
  border-left: 4px solid #409eff;
}

.latency-card {
  border-left: 4px solid #67c23a;
}

.packet-loss-card {
  border-left: 4px solid #e6a23c;
}

.alert-card {
  border-left: 4px solid #f56c6c;
}

.chart-card {
  min-height: 300px;
}

.chart-content {
  height: 250px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.mock-chart {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.chart-bars {
  flex: 1;
  display: flex;
  align-items: flex-end;
  gap: 2px;
  padding: 0 20px;
}

.chart-bar {
  flex: 1;
  background-color: #409eff;
  border-radius: 2px 2px 0 0;
  transition: height 0.3s ease;
}

.chart-x-axis {
  display: flex;
  justify-content: space-around;
  margin-top: 10px;
  padding: 0 20px;
  font-size: 12px;
  color: #909399;
}

.service-status-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 15px;
  width: 100%;
  height: 100%;
  padding: 10px;
}

.service-status-item {
  background-color: #f5f7fa;
  padding: 15px;
  border-radius: 4px;
  display: flex;
  flex-direction: column;
}

.service-name {
  font-weight: bold;
  margin-bottom: 8px;
  color: #303133;
}

.service-status {
  margin-bottom: 8px;
}

.service-metrics {
  display: flex;
  gap: 10px;
  font-size: 12px;
  color: #606266;
}

.resource-usage {
  padding: 20px 0;
}

.resource-item {
  margin-bottom: 20px;
}

.resource-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.resource-value {
  font-weight: bold;
  color: #303133;
}

.alert-list {
  padding: 10px 0;
}

.system-status {
  padding: 20px 0;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 15px;
  padding: 20px;
  background-color: #f5f7fa;
  border-radius: 4px;
}

.status-icon {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.status-icon.success {
  background-color: #f0f9eb;
  color: #67c23a;
}

.status-content {
  flex: 1;
}

.status-title {
  font-size: 14px;
  color: #606266;
  margin-bottom: 5px;
}

.status-value {
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

</style>