<template>

  <div class="app-resource-content">

 <!-- TCP重传详情-->

    <div class="resource-header">

      <div class="resource-search">

        <el-form :inline="true" :model="resourceForm" class="demo-form-inline">

          <el-form-item label="时间范围">

            <el-select v-model="resourceForm.snapshot" placeholder="查询快照" style="width: 200px;">

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="resourceForm.search" placeholder="搜索关键词" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchResource">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="resource-actions">

        <el-form :inline="true" :model="resourceActionsForm" class="demo-form-inline">

          <el-form-item>

            <el-button @click="saveSearch">

              保存搜索条件

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-dropdown>

              <el-button>

                设置

                <el-icon class="el-icon--right"><ArrowDown /></el-icon>

              </el-button>

              <template #dropdown>

                <el-dropdown-menu>

                  <el-dropdown-item @click="showDatabaseFields">数据库字段</el-dropdown-item>

                  <el-dropdown-item @click="toggleTipSync">开启/关闭 Tip 同步</el-dropdown-item>

                  <el-dropdown-item @click="switchInterpolation">切换插值方式</el-dropdown-item>

                  <el-dropdown-item @click="switchStack">切换堆叠模式</el-dropdown-item>

                  <el-dropdown-item @click="switchNameDisplay">切换名称显示/隐藏策略</el-dropdown-item>

                </el-dropdown-menu>

              </template>

            </el-dropdown>

          </el-form-item>

          <el-form-item>

            <el-button @click="refreshResource">

              刷新

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="exportResource">

              导出数据库

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 业务监控-->

    <div class="resource-content">

 <!-- 左侧快速过滤-->

      <div class="resource-sidebar">

 <!-- 应用服务 -->

        <div class="filter-section">

          <h3>应用服务</h3>

          <el-checkbox-group v-model="selectedApps">

            <el-checkbox label="172.25.0.3">172.25.0.3</el-checkbox>

            <el-checkbox label="172.25.0.4">172.25.0.4</el-checkbox>

            <el-checkbox label="bigsch">bigsch</el-checkbox>

            <el-checkbox label="big-nginx">big-nginx</el-checkbox>

            <el-checkbox label="api-server">api-server</el-checkbox>

            <el-checkbox label="big-golang">big-golang</el-checkbox>

            <el-checkbox label="big-nodejs">big-nodejs</el-checkbox>

            <el-checkbox label="big-python">big-python</el-checkbox>

            <el-checkbox label="app-service">app-service</el-checkbox>

          </el-checkbox-group>

        </div>

        

 <!-- 区域查询 -->

        <div class="filter-section">

          <h3>区域查询</h3>

          <el-radio-group v-model="regionQuery">

            <el-radio label="全部">全部</el-radio>

            <el-radio label="区域1">区域1</el-radio>

            <el-radio label="区域2">区域2</el-radio>

            <el-radio label="区域3">区域3</el-radio>

          </el-radio-group>

        </div>

      </div>

      

 <!-- 右侧内容区-->

      <div class="resource-main">

 <!-- 指标选择 -->

        <div class="metrics-selector">

          <el-form :inline="true" :model="resourceMetricsForm" class="demo-form-inline">

            <el-form-item label="指标选择">

              <el-select v-model="resourceMetricsForm.primaryMetric" placeholder="选择默认流量" style="width: 120px;">

                <el-option label="网络流量监控" value="request_rate" />

                <el-option label="服务端错误率" value="server_error" />

                <el-option label="分组聚合详情" value="response_time" />

              </el-select>

            </el-form-item>

            <el-form-item label="分组依据">

              <el-select v-model="resourceMetricsForm.groupBy" placeholder="auto_service" style="width: 120px;">

                <el-option label="auto_service" value="auto_service" />

                <el-option label="主机名" value="host" />

                <el-option label="应用名称" value="app" />

              </el-select>

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="applyResourceMetrics">应用</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 图表数据库展示 -->

        <div class="resource-charts">

          <div class="chart-row">

            <el-card class="chart-card">

              <template #header>

                <div class="chart-header">

                  <h3>网络流量监控</h3>

                </div>

              </template>

              <div class="mock-chart resource-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar resource-bar" :style="{ height: requestRateData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">10:16</div>

                </div>

              </div>

            </el-card>

            <el-card class="chart-card">

              <template #header>

                <div class="chart-header">

                  <h3>服务端错误率</h3>

                </div>

              </template>

              <div class="mock-chart resource-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar resource-bar" :style="{ height: serverErrorData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">10:16</div>

                </div>

              </div>

            </el-card>

            <el-card class="chart-card">

              <template #header>

                <div class="chart-header">

                  <h3>分组聚合详情</h3>

                </div>

              </template>

              <div class="mock-chart resource-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar resource-bar" :style="{ height: responseTimeData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">10:16</div>

                </div>

              </div>

            </el-card>

          </div>

        </div>

        

 <!-- 应用服务列表 -->

        <div class="resource-list">

          <div class="table-header">

            <h3>应用服务列表</h3>

          </div>

          <el-table :data="resourceApps" style="width: 100%" @row-click="handleResourceRowClick">

            <el-table-column prop="service" label="服务" width="120" />

            <el-table-column prop="traffic" label="流量" width="100" />

            <el-table-column prop="requestRate" label="网络流量监控" width="100" />

            <el-table-column prop="errorRate" label="错误率" width="100" />

            <el-table-column prop="clientErrorRate" label="客户端错误率" width="120" />

            <el-table-column prop="serverErrorRate" label="服务端错误率" width="120" />

            <el-table-column prop="avgLatency" label="平均延迟" width="100" />

            <el-table-column prop="p95Latency" label="P95延迟" width="100" />

            <el-table-column prop="qps" label="QPS" width="80" />

            <el-table-column prop="cpuUsage" label="CPU使用率" width="100" />

            <el-table-column prop="memoryUsage" label="内存使用率" width="100" />

            <el-table-column prop="serviceErrorRate" label="服务端错误率" width="120" />

          </el-table>

          

 <!-- 数据库表 -->

          <div class="pagination mt-4">

            <div class="pagination-info">

              共 {{ resourceTotal }} 条

            </div>

            <el-pagination

              background

              layout="prev, pager, next, jumper"

              :total="resourceTotal"

              :page-size="resourcePageSize"

              :current-page="resourceCurrentPage"

              @current-change="handleResourcePageChange"

            />

          </div>

        </div>

      </div>

    </div>

    

 <!-- 抽屉-->

    <el-drawer

      v-model="resourceDrawerVisible"

      title="最近15分钟延迟指标和流量监控数据库"

      direction="rtl"

      size="50%"

    >

      <div class="resource-drawer">

        <h3>服务调用详情</h3>

        <el-descriptions :column="1" border>

          <el-descriptions-item label="服务名称">{{ selectedResourceApp.service }}</el-descriptions-item>

          <el-descriptions-item label="流量">{{ selectedResourceApp.traffic }}</el-descriptions-item>

          <el-descriptions-item label="网络流量监控">{{ selectedResourceApp.requestRate }}</el-descriptions-item>

          <el-descriptions-item label="错误率">{{ selectedResourceApp.errorRate }}</el-descriptions-item>

          <el-descriptions-item label="客户端错误率">{{ selectedResourceApp.clientErrorRate }}</el-descriptions-item>

          <el-descriptions-item label="服务端错误率">{{ selectedResourceApp.serverErrorRate }}</el-descriptions-item>

          <el-descriptions-item label="平均延迟">{{ selectedResourceApp.avgLatency }}</el-descriptions-item>

          <el-descriptions-item label="P95延迟">{{ selectedResourceApp.p95Latency }}</el-descriptions-item>

          <el-descriptions-item label="QPS">{{ selectedResourceApp.qps }}</el-descriptions-item>

          <el-descriptions-item label="CPU使用率">{{ selectedResourceApp.cpuUsage }}</el-descriptions-item>

          <el-descriptions-item label="内存使用率">{{ selectedResourceApp.memoryUsage }}</el-descriptions-item>

          <el-descriptions-item label="服务端错误率">{{ selectedResourceApp.serviceErrorRate }}</el-descriptions-item>

        </el-descriptions>

        <div class="mt-4">

          <h4>关键指标</h4>

          <div class="drawer-charts">

            <div class="drawer-chart">

              <h5>网络流量监控</h5>

              <div class="mock-chart drawer-chart-content">

                <div class="chart-bars">

                  <div v-for="i in 20" :key="i" class="chart-bar drawer-bar" :style="{ height: requestRateData[i-1] + '%' }"></div>

                </div>

              </div>

            </div>

            <div class="drawer-chart">

              <h5>分组聚合详情</h5>

              <div class="mock-chart drawer-chart-content">

                <div class="chart-bars">

                  <div v-for="i in 20" :key="i" class="chart-bar drawer-bar" :style="{ height: responseTimeData[i-1] + '%' }"></div>

                </div>

              </div>

            </div>

          </div>

        </div>

      </div>

    </el-drawer>

  </div>

</template>



<script setup lang="ts">

// 生成模拟数据库（仅在组件挂载时调用一次，避免图表跳动）
const generateMockData = (max: number, min: number, count: number = 30) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

import { ref } from 'vue'

import { ArrowDown } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'



// 资源搜索表单

const resourceForm = ref({

  snapshot: '15m',

  search: ''

})



const resourceActionsForm = ref({})



// 应用服务选择

const selectedApps = ref(['172.25.0.3', '172.25.0.4'])



// 区域查询

const regionQuery = ref('全部')



// 资源分析指标表单

const resourceMetricsForm = ref({

  primaryMetric: 'request_rate',

  groupBy: 'auto_service'

})



// 图表数据库流

const requestRateData = ref([])

const serverErrorData = ref([])

const responseTimeData = ref([])

onMounted(() => {
  requestRateData.value = generateMockData(80, 20, 30)
  serverErrorData.value = generateMockData(50, 5, 30)
  responseTimeData.value = generateMockData(90, 10, 30)
})




// 应用服务列表数据流

const resourceApps = ref([

  {

    service: '172.25.0.3',

    traffic: '445.99',

    requestRate: '393.51',

    errorRate: '10.3K',

    clientErrorRate: '0%',

    serverErrorRate: '5.3 ms',

    avgLatency: '13.56 ms',

    p95Latency: '1.84K',

    qps: '1032.5%',

    cpuUsage: '43.79',

    memoryUsage: '0%',

    serviceErrorRate: '0%'

  },

  {

    service: '172.25.0.4',

    traffic: '442.88',

    requestRate: '271.18',

    errorRate: '10.3K',

    clientErrorRate: '0%',

    serverErrorRate: '56.9 ms',

    avgLatency: '12.7 ms',

    p95Latency: '1.54K',

    qps: '986.3%',

    cpuUsage: '41.6',

    memoryUsage: '0%',

    serviceErrorRate: '0%'

  },

  {

    service: 'bigsch',

    traffic: '401.52',

    requestRate: '478.9',

    errorRate: '20.96K',

    clientErrorRate: '401%',

    serverErrorRate: '598.72 ms',

    avgLatency: '2.32 s',

    p95Latency: '8.85K',

    qps: '4084.89%',

    cpuUsage: '65.94',

    memoryUsage: '2.42%',

    serviceErrorRate: '0%'

  },

  {

    service: 'big-nginx',

    traffic: '398.2',

    requestRate: '227.77',

    errorRate: '847.5',

    clientErrorRate: '0%',

    serverErrorRate: '207.35 ms',

    avgLatency: '1.15 s',

    p95Latency: '5.2K',

    qps: '847.5%',

    cpuUsage: '25.22',

    memoryUsage: '0%',

    serviceErrorRate: '0%'

  },

  {

    service: 'api-server',

    traffic: '310.91',

    requestRate: '184.45',

    errorRate: '844.04',

    clientErrorRate: '640.43%',

    serverErrorRate: '12.94 ms',

    avgLatency: '25.91 s',

    p95Latency: '7.16K',

    qps: '844.64%',

    cpuUsage: '0',

    memoryUsage: '7.98%',

    serviceErrorRate: '0%'

  },

  {

    service: 'big-golang',

    traffic: '257.05',

    requestRate: '198.15',

    errorRate: '49.57',

    clientErrorRate: '200.38%',

    serverErrorRate: '16.7 ms',

    avgLatency: '5.51 s',

    p95Latency: '4.57K',

    qps: '842.57%',

    cpuUsage: '3.58',

    memoryUsage: '1.38%',

    serviceErrorRate: '0%'

  },

  {

    service: 'big-nodejs',

    traffic: '264.25',

    requestRate: '176.96',

    errorRate: '694.86',

    clientErrorRate: '454.35%',

    serverErrorRate: '96.1 ms',

    avgLatency: '798.52 ms',

    p95Latency: '4.31K',

    qps: '824.81%',

    cpuUsage: '1.02',

    memoryUsage: '3.21%',

    serviceErrorRate: '0%'

  },

  {

    service: 'big-python',

    traffic: '208.48',

    requestRate: '143.38',

    errorRate: '301.5',

    clientErrorRate: '1.43%',

    serverErrorRate: '184.42 ms',

    avgLatency: '1.74 s',

    p95Latency: '4.53K',

    qps: '1235%',

    cpuUsage: '0.29',

    memoryUsage: '0%',

    serviceErrorRate: '0%'

  },

  {

    service: 'app-service',

    traffic: '182.04',

    requestRate: '151.04',

    errorRate: '729.07',

    clientErrorRate: '0%',

    serverErrorRate: '8.9 ms',

    avgLatency: '2.05 s',

    p95Latency: '3.96K',

    qps: '78.08%',

    cpuUsage: '0.03',

    memoryUsage: '0.01%',

    serviceErrorRate: '0%'

  }

])



// 数据库表详情

const resourcePageSize = ref(10)

const resourceCurrentPage = ref(1)

const resourceTotal = ref(15)



// 右侧抽屉弹窗

const resourceDrawerVisible = ref(false)

const selectedResourceApp = ref({

  service: '',

  traffic: '',

  requestRate: '',

  errorRate: '',

  clientErrorRate: '',

  serverErrorRate: '',

  avgLatency: '',

  p95Latency: '',

  qps: '',

  cpuUsage: '',

  memoryUsage: '',

  serviceErrorRate: ''

})



// 搜索资源分析

const searchResource = () => {
  ElMessage.info('功能开发中...')
}



// 保存搜索条件

const saveSearch = () => {
  ElMessage.info('功能开发中...')
}



// 显示数据库字段
const showDatabaseFields = () => {
  ElMessage.info('功能开发中...')
}



// 切换Tip同步

const toggleTipSync = () => {
  ElMessage.info('功能开发中...')
}



// 切换插值方式

const switchInterpolation = () => {
  ElMessage.info('功能开发中...')
}



// 切换堆叠模式

const switchStack = () => {
  ElMessage.info('功能开发中...')
}



// 切换策略显示

const switchNameDisplay = () => {
  ElMessage.info('功能开发中...')
}



// 刷新资源分析

const refreshResource = () => {
  ElMessage.info('功能开发中...')
}



// 导出资源分析

const exportResource = () => {
  ElMessage.info('功能开发中...')
}



// 应用资源分析指标

const applyResourceMetrics = () => {
  ElMessage.info('功能开发中...')
}



// 处理资源分析行点击
const handleResourceRowClick = (row: any) => {

  selectedResourceApp.value = row

  resourceDrawerVisible.value = true

  }



// 资源分析分页变化

const handleResourcePageChange = (page: number) => {

  resourceCurrentPage.value = page

  }

</script>



<style scoped>

.app-resource-content {

  padding: 20px;

}



.resource-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.resource-search {

  flex: 1;

}



.resource-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.resource-content {

  display: flex;

  gap: 20px;

}



.resource-sidebar {

  width: 250px;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.filter-section {

  margin-bottom: 20px;

}



.filter-section h3 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.resource-main {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.metrics-selector {

  margin-bottom: 20px;

}



.resource-charts {

  margin-bottom: 30px;

}



.chart-row {

  display: flex;

  gap: 20px;

}



.chart-card {

  flex: 1;

}



.chart-header h3 {

  margin: 0;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.resource-chart {

  height: 200px;

}



.resource-bar {

  background-color: #409eff;

  border-radius: 2px 2px 0 0;

}



.resource-list {

  background-color: #f5f7fa;

  border-radius: 4px;

  padding: 15px;

}



.table-header h3 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.resource-list .el-table {

  margin-bottom: 20px;

}



.resource-list .el-table th {

  background-color: #f5f7fa;

}



.resource-list .el-table td {

  padding: 10px;

}



.pagination {

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.pagination-info {

  color: #909399;

  font-size: 14px;

}



.resource-drawer {

  padding: 20px;

}



.resource-drawer h3 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.resource-drawer h4 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.resource-drawer h5 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 12px;

  font-weight: bold;

  color: #303133;

}



.drawer-charts {

  display: flex;

  gap: 20px;

}



.drawer-chart {

  flex: 1;

}



.drawer-chart-content {

  height: 150px;

}



.drawer-bar {

  background-color: #67c23a;

  border-radius: 2px 2px 0 0;

}



.mt-4 {

  margin-top: 16px;

}



@media (max-width: 1200px) {

  .chart-row {

    flex-direction: column;

  }

  

  .drawer-charts {

    flex-direction: column;

  }

}



/* 模拟图表样式 */

.mock-chart {

  position: relative;

  width: 100%;

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

</style>