<template>

  <div class="service-list">

 <!-- 顶部标签页-->

    <div class="top-tabs">

      <el-tabs v-model="activeTab" class="tab-container">

        <el-tab-pane label="全景" name="overview" />

        <el-tab-pane label="列表" name="list" />

        <el-tab-pane label="拓扑" name="topology" />

      </el-tabs>

      <div class="top-actions">

        <el-form :inline="true" :model="topForm" class="demo-form-inline">

          <el-form-item label="时间范围">

            <el-select v-model="topForm.timeRange" placeholder="选择时间范围" style="width: 120px;">

              <el-option label="最近5分钟" value="5m" />

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-button @click="toggleAutoRefresh">

              {{ autoRefresh ? '关闭自动刷新' : '自动刷新(1s)' }}

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button type="danger" @click="closePage">

              关闭

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 第二行 -->

    <div class="second-row">

      <div class="business-selector">

        <el-form :inline="true" :model="secondForm" class="demo-form-inline">

          <el-form-item label="业务">

            <el-select v-model="secondForm.business" placeholder="选择业务" style="width: 200px;">

              <el-option v-for="business in businessList" :key="business.id" :label="business.name" :value="business.id">

                <template #prefix>

                  <el-icon v-if="business.starred"><StarFilled /></el-icon>

                </template>

                {{ business.name }}

              </el-option>

            </el-select>

          </el-form-item>

        </el-form>

      </div>

      <div class="second-actions">

        <el-form :inline="true" :model="secondForm" class="demo-form-inline">

          <el-form-item>

            <el-input v-model="secondForm.search" placeholder="搜索服务" style="width: 200px;" />

          </el-form-item>

          <el-form-item>

            <el-button @click="openColumnConfig">

              列配置

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-dropdown>

              <el-button>

                更多操作

                <el-icon class="el-icon--right"><ArrowDown /></el-icon>

              </el-button>

              <template #dropdown>

                <el-dropdown-menu>

                  <el-dropdown-item @click="exportData">导出数据库</el-dropdown-item>

                  <el-dropdown-item @click="refreshData">刷新数据库</el-dropdown-item>

                </el-dropdown-menu>

              </template>

            </el-dropdown>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 主数据库表格 -->

    <div class="main-table">

      <el-table 

        :data="serviceData" 

        style="width: 100%"

        @row-dblclick="openServiceDetail"

        height="calc(100vh - 300px)"

      >

        <el-table-column prop="name" label="服务名称" width="180" sortable>

          <template #default="scope">

            <div class="service-name">

              <el-icon class="service-icon"><Operation /></el-icon>

              <span>{{ scope.row.name }}</span>

            </div>

          </template>

        </el-table-column>

        <el-table-column prop="serviceGroup" label="服务分组" width="150" sortable>

          <template #default="scope">

            <div class="service-group">

              <el-icon class="group-icon"><Collection /></el-icon>

              <span>{{ scope.row.serviceGroup }}</span>

            </div>

          </template>

        </el-table-column>

        <el-table-column prop="region" label="区域" width="120" sortable />

        <el-table-column prop="responseTime" label="响应时间" width="150" sortable>

          <template #default="scope">

            <div class="metric-item">

              <div class="metric-bar" :class="{ 'error': scope.row.responseTime > 100 }">

                <div class="metric-progress" :style="{ width: Math.min(scope.row.responseTime / 2, 100) + '%' }"></div>

              </div>

              <div class="metric-value" :class="{ 'error': scope.row.responseTime > 100 }">

                {{ scope.row.responseTime }}ms

              </div>

            </div>

          </template>

        </el-table-column>

        <el-table-column prop="errorRate" label="错误率" width="120" sortable>

          <template #default="scope">

            <div class="metric-item">

              <div class="metric-bar" :class="{ 'error': scope.row.errorRate > 1 }">

                <div class="metric-progress" :style="{ width: Math.min(scope.row.errorRate * 10, 100) + '%' }"></div>

              </div>

              <div class="metric-value" :class="{ 'error': scope.row.errorRate > 1 }">

                {{ scope.row.errorRate }}%

              </div>

            </div>

          </template>

        </el-table-column>

        <el-table-column prop="qps" label="QPS" width="120" sortable>

          <template #default="scope">

            <div class="metric-item">

              <div class="metric-bar">

                <div class="metric-progress" :style="{ width: Math.min(scope.row.qps / 10, 100) + '%' }"></div>

              </div>

              <div class="metric-value">

                {{ scope.row.qps }}

              </div>

            </div>

          </template>

        </el-table-column>

        <el-table-column prop="requestRate" label="请求速率" width="120" sortable>

          <template #default="scope">

            <div class="metric-item">

              <div class="metric-bar">

                <div class="metric-progress" :style="{ width: Math.min(scope.row.requestRate * 10, 100) + '%' }"></div>

              </div>

              <div class="metric-value">

                {{ scope.row.requestRate }}

              </div>

            </div>

          </template>

        </el-table-column>

      </el-table>

    </div>

    

 <!-- 数据库表 -->

    <div class="pagination">

      <div class="pagination-info">

        共 {{ total }} 条

      </div>

      <el-pagination

        background

        layout="prev, pager, next, jumper"

        :total="total"

        :page-size="pageSize"

        :current-page="currentPage"

        @current-change="handlePageChange"

      />

    </div>

    

 <!-- 服务详情抽屉 -->

    <el-drawer

      v-model="serviceDrawerVisible"

      title="服务详情"

      direction="rtl"

      size="50%"

    >

      <div class="service-detail">

        <h4>{{ selectedService.name }} - 服务详情</h4>

        <el-descriptions :column="1" border>

          <el-descriptions-item label="服务名称">{{ selectedService.name }}</el-descriptions-item>

          <el-descriptions-item label="服务分组">{{ selectedService.serviceGroup }}</el-descriptions-item>

          <el-descriptions-item label="区域">{{ selectedService.region }}</el-descriptions-item>

          <el-descriptions-item label="响应时间">{{ selectedService.responseTime }}ms</el-descriptions-item>

          <el-descriptions-item label="错误率">{{ selectedService.errorRate }}%</el-descriptions-item>

          <el-descriptions-item label="QPS">{{ selectedService.qps }}</el-descriptions-item>

          <el-descriptions-item label="请求速率">{{ selectedService.requestRate }}</el-descriptions-item>

        </el-descriptions>

        <div class="mt-4">

          <h5>性能指标</h5>

          <div class="metrics-chart">

            <div class="mock-chart">

              <div class="chart-bars">

                <div v-for="i in 60" :key="i" class="chart-bar" :style="{ height: metricsData[i-1] + '%' }"></div>

              </div>

              <div class="chart-x-axis">

                <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

              </div>

            </div>

          </div>

        </div>

      </div>

    </el-drawer>

    

 <!-- 列配置弹窗 -->

    <el-dialog

      v-model="columnConfigVisible"

      title="列配置"

      width="600px"

    >

      <div class="column-config">

        <h5>修改指标选择</h5>

        <el-checkbox-group v-model="selectedMetrics">

          <el-checkbox v-for="metric in metricOptions" :key="metric.value" :label="metric.value">

            {{ metric.label }}

          </el-checkbox>

        </el-checkbox-group>

        <h5 class="mt-4">主指标选择</h5>

        <el-radio-group v-model="mainMetric">

          <el-radio v-for="metric in metricOptions" :key="metric.value" :label="metric.value">

            {{ metric.label }}

          </el-radio>

        </el-radio-group>

      </div>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="columnConfigVisible = false">取消</el-button>

          <el-button type="primary" @click="saveColumnConfig">确定</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

// 生成模拟数据库（仅在组件挂载时调用一次，避免图表跳动）
const generateMockData = (max: number, min: number, count: number = 30) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Operation, Collection, StarFilled, ArrowDown } from '@element-plus/icons-vue'


// 路由实例
const router = useRouter()

// 活跃标签

const activeTab = ref('list')



// 顶部表单

const topForm = ref({

  timeRange: '5m'

})



// 自动刷新

const autoRefresh = ref(true)

let refreshInterval: number | null = null



// 第二行表单

const secondForm = ref({

  business: 1,

  search: ''

})



// 业务列表

const businessList = ref([

  { id: 1, name: '电商业务', starred: true },

  { id: 2, name: '物流业务', starred: false },

  { id: 3, name: '营销业务', starred: false }

])



// 服务数据库

const serviceData = ref([

  {

    id: 1,

    name: 'web-shop',

    serviceGroup: '服务分组',

    region: '区域1',

    responseTime: 120,

    errorRate: 0.5,

    qps: 1000,

    requestRate: 1.2

  },

  {

    id: 2,

    name: 'svc-user',

    serviceGroup: '服务分组',

    region: '区域1',

    responseTime: 80,

    errorRate: 0.2,

    qps: 800,

    requestRate: 0.9

  },

  {

    id: 3,

    name: 'svc-order',

    serviceGroup: '服务分组',

    region: '区域2',

    responseTime: 95,

    errorRate: 0.8,

    qps: 600,

    requestRate: 0.7

  },

  {

    id: 4,

    name: 'svc-payment',

    serviceGroup: '服务分组',

    region: '区域2',

    responseTime: 150,

    errorRate: 2.0,

    qps: 500,

    requestRate: 0.6

  },

  {

    id: 5,

    name: 'svc-shipping',

    serviceGroup: '服务分组',

    region: '区域3',

    responseTime: 75,

    errorRate: 0.1,

    qps: 400,

    requestRate: 0.5

  }

])



// 分页相关数据库

const pageSize = ref(10)

const currentPage = ref(1)

const total = ref(5)



// 服务详情抽屉

const serviceDrawerVisible = ref(false)

const selectedService = ref({

  name: '',

  serviceGroup: '',

  region: '',

  responseTime: 0,

  errorRate: 0,

  qps: 0,

  requestRate: 0

})



// 性能指标数据库

const metricsData = ref([])



// 列配置

const columnConfigVisible = ref(false)

const metricOptions = ref([

  { label: '分组聚合详情', value: 'responseTime' },

  { label: '错误率', value: 'errorRate' },

  { label: 'QPS', value: 'qps' },

  { label: '网络流量监控', value: 'requestRate' }

])

const selectedMetrics = ref(['responseTime', 'errorRate', 'qps', 'requestRate'])

const mainMetric = ref('responseTime')



// 切换自动刷新

const toggleAutoRefresh = () => {

  autoRefresh.value = !autoRefresh.value

  if (autoRefresh.value) {

    startAutoRefresh()

  } else {

    stopAutoRefresh()

  }

  }



// 开始自动刷新

const startAutoRefresh = () => {

  if (refreshInterval) {

    clearInterval(refreshInterval)

  }

  refreshInterval = window.setInterval(() => {

    refreshData()

  }, 30000)

}



// 停止自动刷新

const stopAutoRefresh = () => {

  if (refreshInterval) {

    clearInterval(refreshInterval)

    refreshInterval = null

  }

}



// 关闭当前页面，返回服务列表
const closePage = () => {
  router.push('/service')
}



// 打开列配置

const openColumnConfig = () => {

  columnConfigVisible.value = true

}



// 保存列配置

const saveColumnConfig = () => {

  columnConfigVisible.value = false

  }



// 导出数据库

const exportData = () => {

 // 实现导出数据库的具体逻辑

}



// 刷新数据库

const refreshData = () => {

 // 实现刷新数据库的具体逻辑

}



// 打开服务详情

const openServiceDetail = (row: any) => {

  selectedService.value = row

  serviceDrawerVisible.value = true

  }



// 分页变化

const handlePageChange = (page: number) => {

  currentPage.value = page

  }



// 生命周期钩子

onMounted(() => {
  metricsData.value = generateMockData(80, 20, 60)

  if (autoRefresh.value) {

    startAutoRefresh()

  }

})



onUnmounted(() => {

  stopAutoRefresh()

})

</script>



<style scoped>

.service-list {

  padding: 24px;

  height: 100%;

  display: flex;

  flex-direction: column;

  gap: 24px;

}



.top-tabs {

  display: flex;

  justify-content: space-between;

  align-items: center;

  background-color: white;

  border-radius: 4px;

  padding: 16px 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.tab-container {

  flex: 1;

}



.top-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.second-row {

  display: flex;

  justify-content: space-between;

  align-items: center;

  background-color: white;

  border-radius: 4px;

  padding: 16px 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.business-selector {

  flex: 1;

}



.second-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.main-table {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  overflow: hidden;

}



.main-table .el-table {

  width: 100%;

}



.main-table .el-table th {

  background-color: #f5f7fa;

  position: sticky;

  top: 0;

  z-index: 10;

}



.service-name {

  display: flex;

  align-items: center;

  gap: 8px;

}



.service-icon {

  color: #1677FF;

}



.service-group {

  display: flex;

  align-items: center;

  gap: 8px;

}



.group-icon {

  color: #67c23a;

}



.metric-item {

  display: flex;

  align-items: center;

  gap: 8px;

  width: 100%;

}



.metric-bar {

  flex: 1;

  height: 8px;

  background-color: #f0f0f0;

  border-radius: 4px;

  overflow: hidden;

  position: relative;

}



.metric-bar.error {

  background-color: #fff1f0;

}



.metric-progress {

  height: 100%;

  background-color: #1677FF;

  border-radius: 4px;

  transition: width 0.3s ease;

}



.metric-bar.error .metric-progress {

  background-color: #FF4D4F;

}



.metric-value {

  font-size: 12px;

  color: #606266;

  min-width: 50px;

  text-align: right;

}



.metric-value.error {

  color: #FF4D4F;

  font-weight: bold;

}



.pagination {

  display: flex;

  justify-content: space-between;

  align-items: center;

  background-color: white;

  border-radius: 4px;

  padding: 16px 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.pagination-info {

  color: #909399;

  font-size: 14px;

}



.service-detail {

  padding: 24px;

}



.service-detail h4 {

  margin-top: 0;

  margin-bottom: 24px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.service-detail h5 {

  margin-top: 0;

  margin-bottom: 16px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.metrics-chart {

  padding: 24px;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

}



.column-config {

  padding: 24px 0;

}



.column-config h5 {

  margin-top: 0;

  margin-bottom: 16px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.mt-4 {

  margin-top: 24px;

}



.dialog-footer {

  display: flex;

  justify-content: flex-end;

  gap: 10px;

}



/* 模拟图表样式 */

.mock-chart {

  position: relative;

  width: 100%;

  height: 200px;

  overflow: hidden;

}



.chart-bars {

  display: flex;

  align-items: flex-end;

  height: 80%;

  gap: 2px;

  padding: 0 10px;

}



.chart-bar {

  flex: 1;

  min-height: 2px;

  background-color: #409eff;

  border-radius: 2px 2px 0 0;

  transition: height 0.3s ease;

}



.chart-x-axis {

  display: flex;

  justify-content: space-between;

  height: 20%;

  padding: 0 10px;

  margin-top: 5px;

}



.x-axis-label {

  font-size: 10px;

  color: #909399;

  text-align: center;

  flex: 1;

}



@media (max-width: 1200px) {

  .top-tabs,

  .second-row {

    flex-direction: column;

    align-items: flex-start;

    gap: 16px;

  }

  

  .pagination {

    flex-direction: column;

    align-items: flex-start;

    gap: 16px;

  }

}



:deep(.el-button--primary) {

  background-color: #1677FF;

  border-color: #1677FF;

}



:deep(.el-button--danger) {

  background-color: #FF4D4F;

  border-color: #FF4D4F;

}



:deep(.el-tabs__active-bar) {

  background-color: #1677FF;

}



:deep(.el-tabs__item.is-active) {

  color: #1677FF;

}

</style>