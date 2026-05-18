<template>
  <div class="business-content">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">首页</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>业务观测</el-breadcrumb-item>
      </el-breadcrumb>
      <h2>业务观测</h2>
    </div>

    <!-- 统计概览 -->
    <div class="overview-section">
      <el-row :gutter="20">
        <el-col :span="6" v-for="item in overviewStats" :key="item.title">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-info">
                <div class="stat-title">{{ item.title }}</div>
                <div class="stat-value">{{ item.value }}</div>
              </div>
              <div class="stat-icon" :style="{ backgroundColor: item.color + '20', color: item.color }">
                <el-icon :size="28"><component :is="item.icon" /></el-icon>
              </div>
            </div>
            <div class="stat-trend" :class="item.trend > 0 ? 'trend-up' : 'trend-down'">
              <el-icon><component :is="item.trend > 0 ? 'Top' : 'Bottom'" /></el-icon>
              {{ Math.abs(item.trend) }}% 较上周
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- 搜索筛选区域 -->
    <div class="filter-section">
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="业务名称">
          <el-input v-model="searchForm.keyword" placeholder="输入业务名称搜索" style="width: 220px;" clearable />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="全部状态" style="width: 140px;" clearable>
            <el-option label="正常" value="normal" />
            <el-option label="告警" value="warning" />
            <el-option label="异常" value="error" />
          </el-select>
        </el-form-item>
        <el-form-item label="时间范围">
          <el-select v-model="searchForm.timeRange" placeholder="查询快照" style="width: 160px;">
            <el-option label="最近15分钟" value="15m" />
            <el-option label="最近30分钟" value="30m" />
            <el-option label="最近1小时" value="1h" />
            <el-option label="最近6小时" value="6h" />
            <el-option label="最近24小时" value="24h" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">
            <el-icon><Search /></el-icon> 搜索
          </el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 业务列表 -->
    <div class="table-section">
      <el-table :data="businessList" style="width: 100%" stripe>
        <el-table-column prop="name" label="业务名称" min-width="150">
          <template #default="scope">
            <el-button type="primary" link @click="viewBusiness(scope.row)">{{ scope.row.name }}</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="serviceCount" label="服务数量" width="100" align="center" />
        <el-table-column prop="endpointCount" label="接入点数量" width="110" align="center" />
        <el-table-column prop="avgLatency" label="平均延迟" width="120" align="center">
          <template #default="scope">
            <span :class="getLatencyClass(scope.row.avgLatency)">{{ scope.row.avgLatency }}ms</span>
          </template>
        </el-table-column>
        <el-table-column prop="errorRate" label="错误率" width="100" align="center">
          <template #default="scope">
            <el-tag :type="getErrorRateType(scope.row.errorRate)" size="small">{{ scope.row.errorRate }}%</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="throughput" label="吞吐量(req/s)" width="130" align="center" />
        <el-table-column prop="status" label="状态" width="80" align="center">
          <template #default="scope">
            <el-tag :type="scope.row.status === '正常' ? 'success' : scope.row.status === '告警' ? 'warning' : 'danger'" size="small">
              {{ scope.row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="updateTime" label="更新时间" width="180" sortable />
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="scope">
            <el-button size="small" link type="primary" @click="viewBusiness(scope.row)">详情</el-button>
            <el-button size="small" link type="primary" @click="viewTopology(scope.row)">拓扑</el-button>
            <el-button size="small" link type="primary" @click="viewServices(scope.row)">服务</el-button>
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
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Search, Top, Bottom, Monitor, Connection, Warning, DataLine } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const router = useRouter()

// 概览统计
const overviewStats = ref([
  { title: '业务总数', value: 12, icon: 'Monitor', color: '#409eff', trend: 8.5 },
  { title: '服务总数', value: 86, icon: 'Connection', color: '#67c23a', trend: 12.3 },
  { title: '平均延迟', value: '23.5ms', icon: 'DataLine', color: '#e6a23c', trend: -5.2 },
  { title: '告警数量', value: 3, icon: 'Warning', color: '#f56c6c', trend: 15.0 }
])

// 搜索表单
const searchForm = ref({
  keyword: '',
  status: '',
  timeRange: '15m'
})

// 业务列表数据
const businessList = ref([
  {
    id: 1,
    name: '电商平台',
    description: '核心电商交易业务，包含商品、订单、支付等模块',
    serviceCount: 18,
    endpointCount: 45,
    avgLatency: 23.5,
    errorRate: 0.12,
    throughput: 1250,
    status: '正常',
    updateTime: '2024-01-15 10:30:00'
  },
  {
    id: 2,
    name: '用户中心',
    description: '用户注册、登录、权限管理等核心用户服务',
    serviceCount: 8,
    endpointCount: 22,
    avgLatency: 15.2,
    errorRate: 0.05,
    throughput: 860,
    status: '正常',
    updateTime: '2024-01-15 10:29:00'
  },
  {
    id: 3,
    name: '支付服务',
    description: '支付网关、退款、对账等支付相关业务',
    serviceCount: 12,
    endpointCount: 30,
    avgLatency: 45.8,
    errorRate: 0.35,
    throughput: 520,
    status: '告警',
    updateTime: '2024-01-15 10:28:00'
  },
  {
    id: 4,
    name: '物流追踪',
    description: '物流信息查询、配送状态跟踪等物流业务',
    serviceCount: 6,
    endpointCount: 15,
    avgLatency: 32.1,
    errorRate: 0.08,
    throughput: 340,
    status: '正常',
    updateTime: '2024-01-15 10:27:00'
  },
  {
    id: 5,
    name: '消息推送',
    description: '短信、邮件、APP推送等消息通知服务',
    serviceCount: 5,
    endpointCount: 12,
    avgLatency: 120.5,
    errorRate: 2.15,
    throughput: 180,
    status: '异常',
    updateTime: '2024-01-15 10:26:00'
  },
  {
    id: 6,
    name: '数据分析',
    description: '用户行为分析、数据报表、BI看板等',
    serviceCount: 10,
    endpointCount: 28,
    avgLatency: 85.3,
    errorRate: 0.22,
    throughput: 95,
    status: '正常',
    updateTime: '2024-01-15 10:25:00'
  },
  {
    id: 7,
    name: '库存管理',
    description: '库存查询、库存变更、库存预警等',
    serviceCount: 7,
    endpointCount: 18,
    avgLatency: 18.7,
    errorRate: 0.03,
    throughput: 420,
    status: '正常',
    updateTime: '2024-01-15 10:24:00'
  },
  {
    id: 8,
    name: '搜索引擎',
    description: '全文检索、商品搜索、推荐服务等',
    serviceCount: 9,
    endpointCount: 20,
    avgLatency: 55.2,
    errorRate: 0.18,
    throughput: 780,
    status: '正常',
    updateTime: '2024-01-15 10:23:00'
  }
])

// 分页
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(8)

// 延迟样式
const getLatencyClass = (latency: number) => {
  if (latency < 50) return 'latency-good'
  if (latency < 100) return 'latency-warning'
  return 'latency-error'
}

// 错误率标签类型
const getErrorRateType = (rate: number) => {
  if (rate < 0.1) return 'success'
  if (rate < 1) return 'warning'
  return 'danger'
}

// 搜索
const handleSearch = () => {
  ElMessage.info('搜索功能开发中...')
}

// 重置
const handleReset = () => {
  searchForm.value = { keyword: '', status: '', timeRange: '15m' }
}

// 分页
const handlePageChange = (page: number) => {
  currentPage.value = page
}

// 查看业务详情
const viewBusiness = (row: any) => {
  router.push(`/business/${row.id}`)
}

// 查看拓扑
const viewTopology = (row: any) => {
  router.push(`/business/topology/${row.id}`)
}

// 查看服务
const viewServices = (row: any) => {
  router.push(`/business/services/${row.id}`)
}
</script>

<style scoped>
.business-content {
  padding: 20px;
  background-color: #f5f7fa;
  min-height: 100%;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 8px 0 0 0;
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

.overview-section {
  margin-bottom: 20px;
}

.stat-card {
  margin-bottom: 10px;
}

.stat-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stat-title {
  font-size: 14px;
  color: #909399;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 24px;
  font-weight: bold;
  color: #303133;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-trend {
  font-size: 12px;
  margin-top: 8px;
}

.trend-up {
  color: #67c23a;
}

.trend-down {
  color: #f56c6c;
}

.filter-section {
  background-color: white;
  border-radius: 4px;
  padding: 16px 20px;
  margin-bottom: 20px;
}

.search-form {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: flex-end;
}

.table-section {
  background-color: white;
  border-radius: 4px;
  padding: 20px;
}

.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 20px;
}

.pagination-info {
  color: #909399;
  font-size: 14px;
}

.latency-good {
  color: #67c23a;
}

.latency-warning {
  color: #e6a23c;
}

.latency-error {
  color: #f56c6c;
}
</style>
