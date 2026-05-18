<template>
  <div class="search-log">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">首页</router-link></el-breadcrumb-item>
        <el-breadcrumb-item><router-link to="/search">搜索中心</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>日志搜索</el-breadcrumb-item>
      </el-breadcrumb>
      <h2>日志搜索</h2>
    </div>

    <!-- 时间范围 -->
    <div class="snapshot-section">
      <SearchSnapshot />
    </div>

    <!-- 日志搜索框 -->
    <div class="log-search-section">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>日志级别过滤</span>
          </div>
        </template>
        <div class="log-search-form">
          <el-form :inline="true" :model="searchForm" class="search-form">
            <el-form-item label="搜索快照管理">
              <el-date-picker
                v-model="searchForm.timeRange"
                type="daterange"
                range-separator="至"
                start-placeholder="开始日期"
                end-placeholder="结束日期"
                style="width: 300px"
              />
            </el-form-item>
            <el-form-item label="日志时间类别">
              <el-select v-model="searchForm.level" placeholder="选择级别">
                <el-option label="全部" value="" />
                <el-option label="DEBUG" value="debug" />
                <el-option label="INFO" value="info" />
                <el-option label="WARN" value="warn" />
                <el-option label="ERROR" value="error" />
                <el-option label="FATAL" value="fatal" />
              </el-select>
            </el-form-item>
            <el-form-item label="关键词">
              <el-input v-model="searchForm.keyword" placeholder="输入关键词" style="width: 300px"></el-input>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="search">
                <el-icon><Search /></el-icon>搜索
              </el-button>
              <el-button @click="clear">
                保存搜索
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-card>
    </div>

    <!-- 日志结果区域 -->
    <div class="result-section">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>搜索结果 (共 {{ total }} 条)</span>
            <div class="result-actions">
              <el-button-group>
                <el-button :type="viewMode === 'table' ? 'primary' : ''" size="small" @click="viewMode = 'table'">
                  <el-icon><List /></el-icon> 列表
                </el-button>
                <el-button :type="viewMode === 'timeline' ? 'primary' : ''" size="small" @click="viewMode = 'timeline'">
                  <el-icon><Clock /></el-icon> 时间线
                </el-button>
              </el-button-group>
              <el-button @click="exportLog">
                <el-icon><Download /></el-icon>
                导出
              </el-button>
            </div>
          </div>
        </template>

        <!-- 列表视图 -->
        <div v-if="viewMode === 'table'">
          <el-table :data="logData" style="width: 100%" stripe @row-click="handleRowClick">
            <el-table-column prop="timestamp" label="时间" width="180" />
            <el-table-column prop="level" label="级别" width="80" align="center">
              <template #default="scope">
                <el-tag :type="getLevelType(scope.row.level)" size="small" effect="dark">
                  {{ scope.row.level }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="service" label="服务" width="150" />
            <el-table-column prop="host" label="主机" width="150" />
            <el-table-column prop="message" label="日志内容" min-width="300" show-overflow-tooltip />
            <el-table-column prop="traceId" label="TraceID" width="180" show-overflow-tooltip />
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
        </div>

        <!-- 时间线视图 -->
        <div v-else class="timeline-view">
          <div v-for="log in logData" :key="log.id" class="timeline-item" @click="handleRowClick(log)">
            <div class="timeline-dot" :class="'dot-' + log.level.toLowerCase()"></div>
            <div class="timeline-content">
              <div class="timeline-header">
                <el-tag :type="getLevelType(log.level)" size="small" effect="dark">{{ log.level }}</el-tag>
                <span class="timeline-service">{{ log.service }}</span>
                <span class="timeline-host">{{ log.host }}</span>
                <span class="timeline-time">{{ log.timestamp }}</span>
              </div>
              <div class="timeline-message">{{ log.message }}</div>
              <div class="timeline-trace" v-if="log.traceId">TraceID: {{ log.traceId }}</div>
            </div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 日志详情抽屉 -->
    <el-drawer v-model="drawerVisible" title="日志详情" direction="rtl" size="50%">
      <div class="log-detail" v-if="selectedLog">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="时间">{{ selectedLog.timestamp }}</el-descriptions-item>
          <el-descriptions-item label="级别">
            <el-tag :type="getLevelType(selectedLog.level)" size="small" effect="dark">{{ selectedLog.level }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="服务">{{ selectedLog.service }}</el-descriptions-item>
          <el-descriptions-item label="主机">{{ selectedLog.host }}</el-descriptions-item>
          <el-descriptions-item label="TraceID">{{ selectedLog.traceId }}</el-descriptions-item>
          <el-descriptions-item label="日志内容">
            <div class="log-content-detail">{{ selectedLog.message }}</div>
          </el-descriptions-item>
        </el-descriptions>
        <div class="detail-actions" style="margin-top: 20px;">
          <el-button type="primary" @click="viewTrace">查看调用链</el-button>
          <el-button @click="copyLog">复制日志</el-button>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import SearchSnapshot from '../components/SearchSnapshot.vue'
import { Search, Download, List, Clock } from '@element-plus/icons-vue'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const searchForm = ref({
  timeRange: [],
  level: '',
  keyword: ''
})

const viewMode = ref('table')

// 日志数据
const logData = ref([
  {
    id: 1,
    timestamp: '2024-01-15 10:30:15.234',
    level: 'ERROR',
    service: 'payment-service',
    host: 'node-03',
    message: 'Payment gateway connection timeout after 30s. Retry attempt 3/3 failed. Order ID: ORD-20240115-00892',
    traceId: 'trace-a1b2c3d4-e5f6-7890-abcd-ef1234567890'
  },
  {
    id: 2,
    timestamp: '2024-01-15 10:30:12.891',
    level: 'WARN',
    service: 'api-gateway',
    host: 'node-01',
    message: 'Request rate limit approaching threshold: 4500/5000 req/s for endpoint /api/v2/orders',
    traceId: 'trace-b2c3d4e5-f6a7-8901-bcde-f12345678901'
  },
  {
    id: 3,
    timestamp: '2024-01-15 10:30:10.456',
    level: 'INFO',
    service: 'user-service',
    host: 'node-02',
    message: 'User login successful. User ID: USR-10234, IP: 192.168.1.100, Session TTL: 3600s',
    traceId: 'trace-c3d4e5f6-a7b8-9012-cdef-123456789012'
  },
  {
    id: 4,
    timestamp: '2024-01-15 10:30:08.123',
    level: 'INFO',
    service: 'order-service',
    host: 'node-02',
    message: 'Order created successfully. Order ID: ORD-20240115-00893, Total: 299.99, Items: 3',
    traceId: 'trace-d4e5f6a7-b8c9-0123-defa-234567890123'
  },
  {
    id: 5,
    timestamp: '2024-01-15 10:30:05.789',
    level: 'ERROR',
    service: 'inventory-service',
    host: 'node-04',
    message: 'Database connection pool exhausted. Active: 50, Idle: 0, Waiting: 12. Query timeout after 60s',
    traceId: 'trace-e5f6a7b8-c9d0-1234-efab-345678901234'
  },
  {
    id: 6,
    timestamp: '2024-01-15 10:30:02.345',
    level: 'WARN',
    service: 'notification-service',
    host: 'node-05',
    message: 'Email delivery delayed. SMTP server response time: 8500ms. Queue size: 234',
    traceId: 'trace-f6a7b8c9-d0e1-2345-fabc-456789012345'
  },
  {
    id: 7,
    timestamp: '2024-01-15 10:29:58.012',
    level: 'DEBUG',
    service: 'search-service',
    host: 'node-06',
    message: 'Elasticsearch query executed. Index: products, Took: 23ms, Hits: 156, Total: 15234',
    traceId: 'trace-a7b8c9d0-e1f2-3456-abcd-567890123456'
  },
  {
    id: 8,
    timestamp: '2024-01-15 10:29:55.678',
    level: 'INFO',
    service: 'auth-service',
    host: 'node-01',
    message: 'Token refreshed successfully. User ID: USR-10567, New token TTL: 7200s',
    traceId: 'trace-b8c9d0e1-f2a3-4567-bcde-678901234567'
  },
  {
    id: 9,
    timestamp: '2024-01-15 10:29:50.234',
    level: 'FATAL',
    service: 'database-service',
    host: 'node-07',
    message: 'Primary database replication lag exceeded threshold: 120s. Failover initiated to replica node-08',
    traceId: 'trace-c9d0e1f2-a3b4-5678-cdef-789012345678'
  },
  {
    id: 10,
    timestamp: '2024-01-15 10:29:45.890',
    level: 'INFO',
    service: 'logistics-service',
    host: 'node-03',
    message: 'Shipment status updated. Tracking: SF1234567890, Status: IN_TRANSIT, From: Beijing, To: Shanghai',
    traceId: 'trace-d0e1f2a3-b4c5-6789-defa-890123456789'
  }
])

// 分页
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(156)

// 抽屉
const drawerVisible = ref(false)
const selectedLog = ref<any>(null)

// 日志级别标签类型
const getLevelType = (level: string) => {
  switch (level) {
    case 'DEBUG': return 'info'
    case 'INFO': return 'success'
    case 'WARN': return 'warning'
    case 'ERROR': return 'danger'
    case 'FATAL': return 'danger'
    default: return ''
  }
}

const search = () => {
  ElMessage.info('搜索功能开发中...')
}

const clear = () => {
  searchForm.value = {
    timeRange: [],
    level: '',
    keyword: ''
  }
}

const exportLog = () => {
  ElMessage.info('导出功能开发中...')
}

const handlePageChange = (page: number) => {
  currentPage.value = page
}

const handleRowClick = (row: any) => {
  selectedLog.value = row
  drawerVisible.value = true
}

const viewTrace = () => {
  ElMessage.info('查看调用链功能开发中...')
}

const copyLog = () => {
  ElMessage.success('日志已复制到剪贴板')
}
</script>

<style scoped>
.search-log {
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

.log-search-section {
  margin-bottom: 24px;
}

.search-form {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: end;
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

/* 时间线视图 */
.timeline-view {
  max-height: 600px;
  overflow-y: auto;
}

.timeline-item {
  display: flex;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background-color 0.2s;
}

.timeline-item:hover {
  background-color: #f5f7fa;
}

.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-top: 5px;
  flex-shrink: 0;
}

.dot-debug { background-color: #909399; }
.dot-info { background-color: #67c23a; }
.dot-warn { background-color: #e6a23c; }
.dot-error { background-color: #f56c6c; }
.dot-fatal { background-color: #f56c6c; width: 12px; height: 12px; margin-top: 4px; }

.timeline-content {
  flex: 1;
}

.timeline-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
}

.timeline-service {
  font-weight: bold;
  color: #303133;
}

.timeline-host {
  color: #606266;
  font-size: 13px;
}

.timeline-time {
  color: #909399;
  font-size: 12px;
  margin-left: auto;
}

.timeline-message {
  color: #303133;
  font-size: 13px;
  line-height: 1.6;
  font-family: monospace;
}

.timeline-trace {
  color: #909399;
  font-size: 12px;
  margin-top: 4px;
}

.log-content-detail {
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.6;
}

:deep(.el-button--primary) {
  background-color: #1677FF;
  border-color: #1677FF;
}
</style>
