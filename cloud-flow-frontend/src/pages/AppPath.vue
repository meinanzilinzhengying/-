<template>

  <div class="app-path-content">

 <!-- TCP重传详情-->

    <div class="path-header">

      <div class="path-search">

        <el-form :inline="true" :model="pathForm" class="demo-form-inline">

          <el-form-item label="时间范围">

            <el-select v-model="pathForm.snapshot" placeholder="查询快照" style="width: 200px;">

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="pathForm.search" placeholder="搜索关键词" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchPath">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="path-actions">

        <el-form :inline="true" :model="pathActionsForm" class="demo-form-inline">

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

            <el-button @click="refreshPath">

              刷新

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="exportPath">

              导出数据库

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 业务监控-->

    <div class="path-content">

 <!-- 左侧快速过滤-->

      <div class="path-sidebar">

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

      <div class="path-main">

 <!-- 指标选择 -->

        <div class="metrics-selector">

          <el-form :inline="true" :model="pathMetricsForm" class="demo-form-inline">

            <el-form-item label="指标选择">

              <el-select v-model="pathMetricsForm.primaryMetric" placeholder="选择默认流量" style="width: 120px;">

                <el-option label="网络流量监控" value="request_rate" />

                <el-option label="服务端错误率" value="server_error" />

                <el-option label="分组聚合详情" value="response_time" />

              </el-select>

            </el-form-item>

            <el-form-item label="分组依据">

              <el-select v-model="pathMetricsForm.groupBy" placeholder="auto_service" style="width: 120px;">

                <el-option label="auto_service" value="auto_service" />

                <el-option label="主机名" value="host" />

                <el-option label="应用名称" value="app" />

              </el-select>

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="applyPathMetrics">应用</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 图表数据库展示 -->

        <div class="path-charts">

          <div class="chart-row">

            <el-card class="chart-card">

              <template #header>

                <div class="chart-header">

                  <h3>网络流量监控</h3>

                </div>

              </template>

              <div class="mock-chart path-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar path-bar" :style="{ height: requestRateData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">11:30</div>

                </div>

              </div>

            </el-card>

            <el-card class="chart-card">

              <template #header>

                <div class="chart-header">

                  <h3>服务错误率</h3>

                </div>

              </template>

              <div class="mock-chart path-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar path-bar" :style="{ height: errorRateData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">11:30</div>

                </div>

              </div>

            </el-card>

            <el-card class="chart-card">

              <template #header>

                <div class="chart-header">

                  <h3>分组聚合详情</h3>

                </div>

              </template>

              <div class="mock-chart path-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar path-bar" :style="{ height: responseTimeData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">11:30</div>

                </div>

              </div>

            </el-card>

          </div>

        </div>

        

 <!-- 路径分析列表 -->

        <div class="path-list">

          <div class="table-header">

            <h3>路径分析列表</h3>

          </div>

          <el-table :data="pathApps" style="width: 100%" @row-click="handlePathRowClick">

            <el-table-column prop="client" label="客户端服务" width="120" />

            <el-table-column prop="server" label="服务端列表" width="120" />

            <el-table-column prop="requestRate" label="网络流量监控" width="100" />

            <el-table-column prop="errorRate" label="错误率" width="100" />

            <el-table-column prop="clientErrorRate" label="客户端错误率" width="120" />

            <el-table-column prop="serverErrorRate" label="服务端错误率" width="120" />

            <el-table-column prop="avgLatency" label="平均延迟" width="100" />

            <el-table-column prop="p95Latency" label="P95延迟" width="100" />

            <el-table-column prop="qps" label="QPS" width="80" />

            <el-table-column prop="count" label="计数" width="80" />

            <el-table-column prop="cost" label="耗时" width="80" />

          </el-table>

          

 <!-- 数据库表 -->

          <div class="pagination mt-4">

            <div class="pagination-info">

              共 {{ pathTotal }} 条

            </div>

            <el-pagination

              background

              layout="prev, pager, next, jumper"

              :total="pathTotal"

              :page-size="pathPageSize"

              :current-page="pathCurrentPage"

              @current-change="handlePathPageChange"

            />

          </div>

        </div>

      </div>

    </div>

    

 <!-- 抽屉-->

    <el-drawer

      v-model="pathDrawerVisible"

      title="路径分析详情"

      direction="rtl"

      size="50%"

    >

      <div class="path-drawer">

        <h3>路径分析详情</h3>

        <el-descriptions :column="1" border>

          <el-descriptions-item label="客户端服务">{{ selectedPathApp.client }}</el-descriptions-item>

          <el-descriptions-item label="服务端列表">{{ selectedPathApp.server }}</el-descriptions-item>

          <el-descriptions-item label="网络流量监控">{{ selectedPathApp.requestRate }}</el-descriptions-item>

          <el-descriptions-item label="错误率">{{ selectedPathApp.errorRate }}</el-descriptions-item>

          <el-descriptions-item label="客户端错误率">{{ selectedPathApp.clientErrorRate }}</el-descriptions-item>

          <el-descriptions-item label="服务端错误率">{{ selectedPathApp.serverErrorRate }}</el-descriptions-item>

          <el-descriptions-item label="平均延迟">{{ selectedPathApp.avgLatency }}</el-descriptions-item>

          <el-descriptions-item label="P95延迟">{{ selectedPathApp.p95Latency }}</el-descriptions-item>

          <el-descriptions-item label="QPS">{{ selectedPathApp.qps }}</el-descriptions-item>

          <el-descriptions-item label="计数">{{ selectedPathApp.count }}</el-descriptions-item>

          <el-descriptions-item label="耗时">{{ selectedPathApp.cost }}</el-descriptions-item>

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



// 路径搜索表单

const pathForm = ref({

  snapshot: '15m',

  search: ''

})



const pathActionsForm = ref({})



// 应用服务选择

const selectedApps = ref(['172.25.0.3', '172.25.0.4'])



// 区域查询

const regionQuery = ref('全部')



// 路径分析指标表单

const pathMetricsForm = ref({

  primaryMetric: 'request_rate',

  groupBy: 'auto_service'

})



// 图表数据库流

const requestRateData = ref([])

const errorRateData = ref([])

const responseTimeData = ref([])

onMounted(() => {
  requestRateData.value = generateMockData(80, 20, 30)
  errorRateData.value = generateMockData(50, 5, 30)
  responseTimeData.value = generateMockData(90, 10, 30)
})




// 路径分析列表加载数据

const pathApps = ref([

  {

    client: '172.25.0.3',

    server: '172.25.0.4',

    requestRate: '2.19K',

    errorRate: '0%',

    clientErrorRate: '0%',

    serverErrorRate: '234.57 us',

    avgLatency: '255.14 us',

    p95Latency: '390.48 us',

    qps: '13',

    count: '55.56K',

    cost: '14.95'

  },

  {

    client: '127.0.0.1',

    server: '172.25.0.3',

    requestRate: '208.02',

    errorRate: '0%',

    clientErrorRate: '0%',

    serverErrorRate: '58.5 ms',

    avgLatency: '58.5 ms',

    p95Latency: '158.32 ms',

    qps: '6.26K',

    count: '26.34K',

    cost: '0'

  },

  {

    client: '172.25.0.4',

    server: '172.25.0.3',

    requestRate: '228.68',

    errorRate: '0%',

    clientErrorRate: '0.07%',

    serverErrorRate: '56.24 ms',

    avgLatency: '56.24 ms',

    p95Latency: '65.73 ms',

    qps: '5.33K',

    count: '22.27K',

    cost: '0.07'

  },

  {

    client: '172.25.0.3',

    server: '172.25.0.2',

    requestRate: '196.86',

    errorRate: '0%',

    clientErrorRate: '0%',

    serverErrorRate: '3.01 ms',

    avgLatency: '3.01 ms',

    p95Latency: '5.93 ms',

    qps: '3.24K',

    count: '13.5K',

    cost: '38.64'

  },

  {

    client: '172.25.0.2',

    server: '172.25.0.1',

    requestRate: '173.54',

    errorRate: '0%',

    clientErrorRate: '2%',

    serverErrorRate: '1.02 ms',

    avgLatency: '1.02 ms',

    p95Latency: '3.07 ms',

    qps: '3.24K',

    count: '13.5K',

    cost: '402.31'

  },

  {

    client: '172.25.0.1',

    server: '172.25.0.5',

    requestRate: '142.79',

    errorRate: '0%',

    clientErrorRate: '0%',

    serverErrorRate: '98.79 ms',

    avgLatency: '98.79 ms',

    p95Latency: '156.27 ms',

    qps: '4.38K',

    count: '18.25K',

    cost: '8.3'

  },

  {

    client: '172.25.0.5',

    server: '172.25.0.6',

    requestRate: '108.16',

    errorRate: '0%',

    clientErrorRate: '0%',

    serverErrorRate: '42.74 ms',

    avgLatency: '42.74 ms',

    p95Latency: '186.72 ms',

    qps: '2.28K',

    count: '9.5K',

    cost: '0'

  },

  {

    client: '172.25.0.6',

    server: '172.25.0.7',

    requestRate: '99.14',

    errorRate: '0%',

    clientErrorRate: '0.65%',

    serverErrorRate: '49.48 ms',

    avgLatency: '49.48 ms',

    p95Latency: '84.23 ms',

    qps: '2.15K',

    count: '8.98K',

    cost: '0.86'

  },

  {

    client: '172.25.0.7',

    server: '172.25.0.8',

    requestRate: '85.71',

    errorRate: '0%',

    clientErrorRate: '0%',

    serverErrorRate: '56.64 ms',

    avgLatency: '56.64 ms',

    p95Latency: '192.88 ms',

    qps: '2.73K',

    count: '11.38K',

    cost: '0'

  }

])



// 数据库表详情

const pathPageSize = ref(10)

const pathCurrentPage = ref(1)

const pathTotal = ref(15)



// 右侧抽屉弹窗

const pathDrawerVisible = ref(false)

const selectedPathApp = ref({

  client: '',

  server: '',

  requestRate: '',

  errorRate: '',

  clientErrorRate: '',

  serverErrorRate: '',

  avgLatency: '',

  p95Latency: '',

  qps: '',

  count: '',

  cost: ''

})



// 搜索路径拓扑图

const searchPath = () => {
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



// 刷新路径分析

const refreshPath = () => {
  ElMessage.info('功能开发中...')
}



// 导出路径拓扑图

const exportPath = () => {
  ElMessage.info('功能开发中...')
}



// 应用路径分析服务调用

const applyPathMetrics = () => {
  ElMessage.info('功能开发中...')
}



// 处理路径分析行点击
const handlePathRowClick = (row: any) => {

  selectedPathApp.value = row

  pathDrawerVisible.value = true

  }



// 路径分析分页变化

const handlePathPageChange = (page: number) => {

  pathCurrentPage.value = page

  }

</script>



<style scoped>

.app-path-content {

  padding: 20px;

}



.path-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.path-search {

  flex: 1;

}



.path-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.path-content {

  display: flex;

  gap: 20px;

}



.path-sidebar {

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



.path-main {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.metrics-selector {

  margin-bottom: 20px;

}



.path-charts {

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



.path-chart {

  height: 200px;

}



.path-bar {

  background-color: #409eff;

  border-radius: 2px 2px 0 0;

}



.path-list {

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



.path-list .el-table {

  margin-bottom: 20px;

}



.path-list .el-table th {

  background-color: #f5f7fa;

}



.path-list .el-table td {

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



.path-drawer {

  padding: 20px;

}



.path-drawer h3 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.path-drawer h4 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.path-drawer h5 {

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