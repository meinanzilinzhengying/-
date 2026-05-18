<template>
  <div class="search-resource">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">管理后台</router-link></el-breadcrumb-item>
        <el-breadcrumb-item><router-link to="/search">数据库搜索</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>资源搜索</el-breadcrumb-item>
      </el-breadcrumb>
      <h2>资源搜索</h2>
    </div>

    <!-- 搜索快照 -->
    <div class="snapshot-section">
      <SearchSnapshot />
    </div>

    <!-- 资源搜索框 -->
    <div class="resource-search-section">
      <ResourceSearch />
    </div>

    <!-- 资源统计概览 -->
    <div class="resource-overview">
      <el-row :gutter="16">
        <el-col :span="4" v-for="stat in resourceStats" :key="stat.label">
          <div class="stat-card">
            <div class="stat-label">{{ stat.label }}</div>
            <div class="stat-value" :style="{ color: stat.color }">{{ stat.value }}</div>
          </div>
        </el-col>
      </el-row>
    </div>

    <!-- 搜索结果 -->
    <div class="result-section">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>搜索结果 (共 {{ total }} 条)</span>
            <div class="result-actions">
              <el-select v-model="resourceType" placeholder="资源类型" size="small" style="width: 140px;" clearable>
                <el-option label="主机" value="host" />
                <el-option label="进程" value="process" />
                <el-option label="容器" value="container" />
                <el-option label="Pod" value="pod" />
                <el-option label="服务" value="service" />
              </el-select>
              <el-button size="small" @click="exportResources">
                <el-icon><Download /></el-icon> 导出
              </el-button>
            </div>
          </div>
        </template>

        <el-table :data="resourceData" style="width: 100%" stripe @row-click="handleRowClick">
          <el-table-column prop="name" label="资源名称" min-width="180">
            <template #default="scope">
              <el-button type="primary" link @click.stop="viewResource(scope.row)">{{ scope.row.name }}</el-button>
            </template>
          </el-table-column>
          <el-table-column prop="type" label="类型" width="100" align="center">
            <template #default="scope">
              <el-tag :type="getResourceTypeTag(scope.row.type)" size="small">{{ scope.row.type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="ip" label="IP地址" width="150" />
          <el-table-column prop="status" label="状态" width="80" align="center">
            <template #default="scope">
              <span class="status-dot" :class="'status-' + scope.row.status"></span>
              {{ scope.row.statusText }}
            </template>
          </el-table-column>
          <el-table-column prop="cpuUsage" label="CPU使用率" width="120" align="center">
            <template #default="scope">
              <el-progress :percentage="scope.row.cpuUsage" :color="getProgressColor(scope.row.cpuUsage)" :stroke-width="6" />
            </template>
          </el-table-column>
          <el-table-column prop="memoryUsage" label="内存使用率" width="120" align="center">
            <template #default="scope">
              <el-progress :percentage="scope.row.memoryUsage" :color="getProgressColor(scope.row.memoryUsage)" :stroke-width="6" />
            </template>
          </el-table-column>
          <el-table-column prop="networkIn" label="网络流入" width="120" align="center">
            <template #default="scope">
              {{ scope.row.networkIn }} MB/s
            </template>
          </el-table-column>
          <el-table-column prop="networkOut" label="网络流出" width="120" align="center">
            <template #default="scope">
              {{ scope.row.networkOut }} MB/s
            </template>
          </el-table-column>
          <el-table-column prop="labels" label="标签" min-width="150" show-overflow-tooltip />
        </el-table>

        <div class="pagination">
          <div class="pagination-info">共 {{ total }} 条</div>
          <el-pagination
            background
            layout="prev, pager, next, jumper"
            :total="total"
            :page-size="pageSize"
            :current-page="currentPage"
            @current-change="handlePageChange"
          />
        </div>
      </el-card>
    </div>

    <!-- 资源详情抽屉 -->
    <el-drawer v-model="drawerVisible" title="资源详情" direction="rtl" size="50%">
      <div class="resource-detail" v-if="selectedResource">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="资源名称">{{ selectedResource.name }}</el-descriptions-item>
          <el-descriptions-item label="类型">
            <el-tag :type="getResourceTypeTag(selectedResource.type)" size="small">{{ selectedResource.type }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="IP地址">{{ selectedResource.ip }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <span class="status-dot" :class="'status-' + selectedResource.status"></span>
            {{ selectedResource.statusText }}
          </el-descriptions-item>
          <el-descriptions-item label="CPU使用率">{{ selectedResource.cpuUsage }}%</el-descriptions-item>
          <el-descriptions-item label="内存使用率">{{ selectedResource.memoryUsage }}%</el-descriptions-item>
          <el-descriptions-item label="网络流入">{{ selectedResource.networkIn }} MB/s</el-descriptions-item>
          <el-descriptions-item label="网络流出">{{ selectedResource.networkOut }} MB/s</el-descriptions-item>
          <el-descriptions-item label="标签" :span="2">{{ selectedResource.labels }}</el-descriptions-item>
        </el-descriptions>

        <div class="detail-actions" style="margin-top: 20px;">
          <el-button type="primary" @click="viewMetrics">查看指标</el-button>
          <el-button @click="viewTopology">查看拓扑</el-button>
          <el-button @click="viewLogs">查看日志</el-button>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import SearchSnapshot from '../components/SearchSnapshot.vue'
import ResourceSearch from '../components/ResourceSearch.vue'
import { Download } from '@element-plus/icons-vue'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const resourceType = ref('')

// 资源统计
const resourceStats = ref([
  { label: '主机', value: '24', color: '#409eff' },
  { label: '进程', value: '386', color: '#67c23a' },
  { label: '容器', value: '152', color: '#e6a23c' },
  { label: 'Pod', value: '148', color: '#909399' },
  { label: '服务', value: '42', color: '#f56c6c' },
  { label: '告警资源', value: '8', color: '#f56c6c' }
])

// 资源数据
const resourceData = ref([
  { name: 'k8s-node-01', type: '主机', ip: '10.0.1.101', status: 'running', statusText: '运行中', cpuUsage: 72, memoryUsage: 68, networkIn: 125.6, networkOut: 85.2, labels: 'env=production,region=cn-east,az=az1' },
  { name: 'k8s-node-02', type: '主机', ip: '10.0.1.102', status: 'running', statusText: '运行中', cpuUsage: 45, memoryUsage: 82, networkIn: 98.4, networkOut: 72.1, labels: 'env=production,region=cn-east,az=az2' },
  { name: 'k8s-node-03', type: '主机', ip: '10.0.1.103', status: 'warning', statusText: '告警', cpuUsage: 92, memoryUsage: 88, networkIn: 256.3, networkOut: 180.5, labels: 'env=production,region=cn-east,az=az1' },
  { name: 'order-service-pod-abc12', type: 'Pod', ip: '10.244.1.15', status: 'running', statusText: '运行中', cpuUsage: 35, memoryUsage: 52, networkIn: 45.2, networkOut: 38.6, labels: 'app=order-service,version=v2.1.0' },
  { name: 'payment-service-main', type: '进程', ip: '10.244.2.8', status: 'running', statusText: '运行中', cpuUsage: 55, memoryUsage: 45, networkIn: 22.1, networkOut: 18.9, labels: 'app=payment-service,pid=12345' },
  { name: 'redis-cache-01', type: '容器', ip: '10.244.3.22', status: 'running', statusText: '运行中', cpuUsage: 15, memoryUsage: 78, networkIn: 320.5, networkOut: 280.3, labels: 'app=redis,role=cache' },
  { name: 'mysql-primary', type: '容器', ip: '10.244.4.10', status: 'running', statusText: '运行中', cpuUsage: 68, memoryUsage: 85, networkIn: 180.2, networkOut: 150.8, labels: 'app=mysql,role=primary' },
  { name: 'nginx-ingress-ctrl', type: '容器', ip: '10.244.0.5', status: 'running', statusText: '运行中', cpuUsage: 28, memoryUsage: 35, networkIn: 520.8, networkOut: 480.2, labels: 'app=ingress-nginx,component=controller' },
  { name: 'k8s-node-04', type: '主机', ip: '10.0.1.104', status: 'stopped', statusText: '已停止', cpuUsage: 0, memoryUsage: 0, networkIn: 0, networkOut: 0, labels: 'env=production,region=cn-east,az=az2' },
  { name: 'elasticsearch-data-0', type: '容器', ip: '10.244.5.12', status: 'warning', statusText: '告警', cpuUsage: 88, memoryUsage: 92, networkIn: 95.3, networkOut: 78.6, labels: 'app=elasticsearch,role=data' }
])

const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(752)

// 抽屉
const drawerVisible = ref(false)
const selectedResource = ref<any>(null)

const getResourceTypeTag = (type: string) => {
  switch (type) {
    case '主机': return 'primary'
    case '进程': return 'success'
    case '容器': return 'warning'
    case 'Pod': return 'info'
    case '服务': return 'danger'
    default: return ''
  }
}

const getProgressColor = (percentage: number) => {
  if (percentage < 60) return '#67c23a'
  if (percentage < 80) return '#e6a23c'
  return '#f56c6c'
}

const handlePageChange = (page: number) => {
  currentPage.value = page
}

const handleRowClick = (row: any) => {
  selectedResource.value = row
  drawerVisible.value = true
}

const viewResource = (row: any) => {
  selectedResource.value = row
  drawerVisible.value = true
}

const viewMetrics = () => {
  ElMessage.info('查看指标功能开发中...')
}

const viewTopology = () => {
  ElMessage.info('查看拓扑功能开发中...')
}

const viewLogs = () => {
  ElMessage.info('查看日志功能开发中...')
}

const exportResources = () => {
  ElMessage.info('导出功能开发中...')
}
</script>

<style scoped>
.search-resource {
  background-color: white;
  border-radius: 4px;
  padding: 24px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  padding-bottom: 16px;
  border-bottom: 1px solid #e4e7ed;
}

.page-header h2 {
  margin: 8px 0 0 0;
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

.snapshot-section {
  margin-bottom: 16px;
}

.resource-search-section {
  margin-bottom: 24px;
}

.resource-overview {
  margin-bottom: 24px;
}

.stat-card {
  background-color: #f5f7fa;
  border-radius: 4px;
  padding: 14px;
  text-align: center;
}

.stat-label {
  font-size: 12px;
  color: #909399;
  margin-bottom: 6px;
}

.stat-value {
  font-size: 20px;
  font-weight: bold;
}

.result-section {
  flex: 1;
  overflow: auto;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.result-actions {
  display: flex;
  gap: 10px;
  align-items: center;
}

.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
}

.pagination-info {
  color: #909399;
  font-size: 14px;
}

/* 状态指示点 */
.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
}

.status-running { background-color: #67c23a; }
.status-warning { background-color: #e6a23c; }
.status-stopped { background-color: #909399; }
.status-error { background-color: #f56c6c; }
</style>
