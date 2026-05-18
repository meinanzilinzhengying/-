<template>
  <div class="search-path">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">管理后台</router-link></el-breadcrumb-item>
        <el-breadcrumb-item><router-link to="/search">数据库搜索</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>路径搜索</el-breadcrumb-item>
      </el-breadcrumb>
      <h2>路径搜索</h2>
    </div>

    <!-- 搜索快照 -->
    <div class="snapshot-section">
      <SearchSnapshot />
    </div>

    <!-- 路径搜索框 -->
    <div class="path-search-section">
      <PathSearch />
    </div>

    <!-- 路径统计概览 -->
    <div class="path-overview">
      <el-row :gutter="16">
        <el-col :span="6" v-for="stat in pathStats" :key="stat.label">
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
              <el-select v-model="sortField" placeholder="排序方式" size="small" style="width: 140px;">
                <el-option label="按延迟排序" value="latency" />
                <el-option label="按吞吐量排序" value="throughput" />
                <el-option label="按错误率排序" value="errorRate" />
              </el-select>
              <el-button size="small" @click="exportPaths">
                <el-icon><Download /></el-icon> 导出
              </el-button>
            </div>
          </div>
        </template>

        <el-table :data="pathData" style="width: 100%" stripe @row-click="handleRowClick">
          <el-table-column prop="path" label="请求路径" min-width="250">
            <template #default="scope">
              <el-button type="primary" link @click.stop="viewPathDetail(scope.row)">{{ scope.row.path }}</el-button>
            </template>
          </el-table-column>
          <el-table-column prop="method" label="方法" width="80" align="center">
            <template #default="scope">
              <el-tag :type="getMethodType(scope.row.method)" size="small" effect="dark">{{ scope.row.method }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="service" label="服务" width="150" />
          <el-table-column prop="avgLatency" label="平均延迟" width="120" align="center">
            <template #default="scope">
              <span :class="getLatencyClass(scope.row.avgLatency)">{{ scope.row.avgLatency }}ms</span>
            </template>
          </el-table-column>
          <el-table-column prop="p99Latency" label="P99延迟" width="120" align="center">
            <template #default="scope">
              <span :class="getLatencyClass(scope.row.p99Latency)">{{ scope.row.p99Latency }}ms</span>
            </template>
          </el-table-column>
          <el-table-column prop="throughput" label="吞吐量(req/s)" width="130" align="center" />
          <el-table-column prop="errorRate" label="错误率" width="100" align="center">
            <template #default="scope">
              <el-tag :type="getErrorRateType(scope.row.errorRate)" size="small">{{ scope.row.errorRate }}%</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="80" align="center">
            <template #default="scope">
              <el-tag :type="scope.row.status === '正常' ? 'success' : scope.row.status === '慢调用' ? 'warning' : 'danger'" size="small">
                {{ scope.row.status }}
              </el-tag>
            </template>
          </el-table-column>
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

    <!-- 路径详情抽屉 -->
    <el-drawer v-model="drawerVisible" title="路径详情" direction="rtl" size="50%">
      <div class="path-detail" v-if="selectedPath">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="请求路径" :span="2">{{ selectedPath.path }}</el-descriptions-item>
          <el-descriptions-item label="请求方法">{{ selectedPath.method }}</el-descriptions-item>
          <el-descriptions-item label="所属服务">{{ selectedPath.service }}</el-descriptions-item>
          <el-descriptions-item label="平均延迟">{{ selectedPath.avgLatency }}ms</el-descriptions-item>
          <el-descriptions-item label="P99延迟">{{ selectedPath.p99Latency }}ms</el-descriptions-item>
          <el-descriptions-item label="吞吐量">{{ selectedPath.throughput }} req/s</el-descriptions-item>
          <el-descriptions-item label="错误率">{{ selectedPath.errorRate }}%</el-descriptions-item>
        </el-descriptions>

        <h4 style="margin: 20px 0 12px;">调用链路</h4>
        <div class="trace-chain">
          <div v-for="(step, index) in traceChain" :key="index" class="trace-step">
            <div class="trace-node">
              <div class="trace-name">{{ step.service }}</div>
              <div class="trace-info">{{ step.duration }}ms</div>
            </div>
            <div v-if="index < traceChain.length - 1" class="trace-arrow">
              <el-icon><ArrowRight /></el-icon>
            </div>
          </div>
        </div>

        <div style="margin-top: 20px;">
          <el-button type="primary" @click="viewTracing">查看完整调用链</el-button>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import SearchSnapshot from '../components/SearchSnapshot.vue'
import PathSearch from '../components/PathSearch.vue'
import { Download, ArrowRight } from '@element-plus/icons-vue'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const sortField = ref('latency')

// 路径统计
const pathStats = ref([
  { label: '路径总数', value: '326', color: '#409eff' },
  { label: '慢调用路径', value: '18', color: '#e6a23c' },
  { label: '异常路径', value: '5', color: '#f56c6c' },
  { label: '平均响应时间', value: '45.2ms', color: '#67c23a' }
])

// 路径数据
const pathData = ref([
  { path: '/api/v2/orders', method: 'POST', service: 'order-service', avgLatency: 125.6, p99Latency: 450.2, throughput: 320, errorRate: 0.35, status: '慢调用' },
  { path: '/api/v2/products/search', method: 'GET', service: 'search-service', avgLatency: 85.3, p99Latency: 230.1, throughput: 890, errorRate: 0.12, status: '正常' },
  { path: '/api/v2/users/login', method: 'POST', service: 'auth-service', avgLatency: 45.2, p99Latency: 120.5, throughput: 560, errorRate: 0.08, status: '正常' },
  { path: '/api/v2/payments/process', method: 'POST', service: 'payment-service', avgLatency: 230.8, p99Latency: 890.5, throughput: 180, errorRate: 2.15, status: '异常' },
  { path: '/api/v2/inventory/check', method: 'GET', service: 'inventory-service', avgLatency: 35.6, p99Latency: 85.2, throughput: 1200, errorRate: 0.03, status: '正常' },
  { path: '/api/v2/shipping/track', method: 'GET', service: 'logistics-service', avgLatency: 68.9, p99Latency: 180.3, throughput: 420, errorRate: 0.15, status: '正常' },
  { path: '/api/v2/notifications/send', method: 'POST', service: 'notification-service', avgLatency: 156.3, p99Latency: 520.8, throughput: 95, errorRate: 1.85, status: '异常' },
  { path: '/api/v2/reports/generate', method: 'POST', service: 'report-service', avgLatency: 890.5, p99Latency: 2500.3, throughput: 12, errorRate: 0.5, status: '慢调用' },
  { path: '/api/v2/cart/items', method: 'GET', service: 'cart-service', avgLatency: 22.1, p99Latency: 55.8, throughput: 2100, errorRate: 0.02, status: '正常' },
  { path: '/api/v2/analytics/events', method: 'POST', service: 'analytics-service', avgLatency: 42.8, p99Latency: 110.2, throughput: 3500, errorRate: 0.05, status: '正常' }
])

const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(326)

// 抽屉
const drawerVisible = ref(false)
const selectedPath = ref<any>(null)

const traceChain = ref([
  { service: 'api-gateway', duration: 5 },
  { service: 'order-service', duration: 85 },
  { service: 'inventory-service', duration: 25 },
  { service: 'payment-service', duration: 180 }
])

const getMethodType = (method: string) => {
  switch (method) {
    case 'GET': return 'success'
    case 'POST': return 'primary'
    case 'PUT': return 'warning'
    case 'DELETE': return 'danger'
    default: return ''
  }
}

const getLatencyClass = (latency: number) => {
  if (latency < 100) return 'latency-good'
  if (latency < 300) return 'latency-warning'
  return 'latency-error'
}

const getErrorRateType = (rate: number) => {
  if (rate < 0.1) return 'success'
  if (rate < 1) return 'warning'
  return 'danger'
}

const handlePageChange = (page: number) => {
  currentPage.value = page
}

const handleRowClick = (row: any) => {
  selectedPath.value = row
  drawerVisible.value = true
}

const viewPathDetail = (row: any) => {
  selectedPath.value = row
  drawerVisible.value = true
}

const viewTracing = () => {
  ElMessage.info('查看完整调用链功能开发中...')
}

const exportPaths = () => {
  ElMessage.info('导出功能开发中...')
}
</script>

<style scoped>
.search-path {
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

.path-search-section {
  margin-bottom: 24px;
}

.path-overview {
  margin-bottom: 24px;
}

.stat-card {
  background-color: #f5f7fa;
  border-radius: 4px;
  padding: 16px;
  text-align: center;
}

.stat-label {
  font-size: 13px;
  color: #909399;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 22px;
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

.latency-good { color: #67c23a; }
.latency-warning { color: #e6a23c; }
.latency-error { color: #f56c6c; }

/* 调用链路 */
.trace-chain {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px;
  background-color: #f5f7fa;
  border-radius: 4px;
  flex-wrap: wrap;
}

.trace-step {
  display: flex;
  align-items: center;
  gap: 8px;
}

.trace-node {
  background-color: white;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  padding: 10px 16px;
  text-align: center;
}

.trace-name {
  font-size: 13px;
  font-weight: bold;
  color: #303133;
}

.trace-info {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.trace-arrow {
  color: #909399;
}
</style>
