<template>

  <div class="app-tracing-content">

 <!-- TCP重传详情-->

    <div class="tracing-header">

      <div class="tracing-search">

        <el-form :inline="true" :model="tracingForm" class="demo-form-inline">

          <el-form-item label="时间范围">

            <el-select v-model="tracingForm.snapshot" placeholder="查询快照" style="width: 200px;">

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="tracingForm.search" placeholder="搜索关键词" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchTracing">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="tracing-actions">

        <el-form :inline="true" :model="tracingActionsForm" class="demo-form-inline">

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

                  <el-dropdown-item @click="switchNameDisplay">切换名称显示/隐藏策略</el-dropdown-item>

                </el-dropdown-menu>

              </template>

            </el-dropdown>

          </el-form-item>

          <el-form-item>

            <el-button @click="refreshTracing">

              刷新

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="exportTracing">

              导出数据库

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 业务监控-->

    <div class="tracing-content">

 <!-- 左侧快速过滤-->

      <div class="tracing-sidebar">

 <!-- 信号源-->

        <div class="filter-section">

          <h3>信号源</h3>

          <el-checkbox-group v-model="selectedSignalSources">

            <el-checkbox label="Packet">Packet</el-checkbox>

            <el-checkbox label="eBPF">eBPF</el-checkbox>

            <el-checkbox label="OTel">OTel</el-checkbox>

          </el-checkbox-group>

        </div>

        

 <!-- 响应状态筛选-->

        <div class="filter-section">

          <h3>响应状态筛选</h3>

          <el-checkbox-group v-model="selectedStatuses">

            <el-checkbox label="正常">正常</el-checkbox>

            <el-checkbox label="警告">警告</el-checkbox>

            <el-checkbox label="服务端错误率">服务端错误率</el-checkbox>

            <el-checkbox label="客户端错误">客户端错误</el-checkbox>

          </el-checkbox-group>

        </div>

        

 <!-- 应用协议 -->

        <div class="filter-section">

          <h3>应用协议</h3>

          <el-checkbox-group v-model="selectedProtocols">

            <el-checkbox label="MySQL">MySQL</el-checkbox>

            <el-checkbox label="HTTP">HTTP</el-checkbox>

            <el-checkbox label="gRPC">gRPC</el-checkbox>

            <el-checkbox label="DNS">DNS</el-checkbox>

            <el-checkbox label="NTP">NTP</el-checkbox>

            <el-checkbox label="HTTP2">HTTP2</el-checkbox>

            <el-checkbox label="Redis">Redis</el-checkbox>

            <el-checkbox label="TLS">TLS</el-checkbox>

            <el-checkbox label="AMQP">AMQP</el-checkbox>

            <el-checkbox label="Dubbo">Dubbo</el-checkbox>

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

      <div class="tracing-main">

 <!-- 图表数据库展示 -->

        <div class="tracing-chart">

          <el-card>

            <template #header>

              <div class="chart-header">

                <h3>异常分析</h3>

              </div>

            </template>

            <div class="mock-chart tracing-chart-content">

              <div class="chart-bars">

                <div v-for="i in 60" :key="i" class="chart-bar tracing-bar" :style="{ height: trendData[i-1] + '%' }"></div>

              </div>

              <div class="chart-x-axis">

                <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

              </div>

            </div>

          </el-card>

        </div>

        

 <!-- 调用链追踪详情 -->

        <div class="tracing-list">

          <div class="table-header">

            <h3>调用链追踪详情</h3>

            <div class="table-actions">

              <el-dropdown>

                <el-button size="small">

                  列选择

                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>

                </el-button>

                <template #dropdown>

                  <el-dropdown-menu>

                    <el-dropdown-item @click="toggleColumn('startTime')">开始时间</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('client')">客户端服务</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('server')">服务端列表</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('requestType')">请求类型</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('requestDomain')">处理域名</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('traceId')">调用链追踪ID</el-dropdown-item>

                  </el-dropdown-menu>

                </template>

              </el-dropdown>

            </div>

          </div>

          <el-table :data="tracingData" style="width: 100%" @row-click="handleTracingRowClick">

            <el-table-column prop="startTime" label="开始时间" width="180" />

            <el-table-column prop="client" label="客户端服务" width="180" />

            <el-table-column prop="server" label="服务端列表" width="180" />

            <el-table-column prop="requestType" label="请求类型" width="120" />

            <el-table-column prop="requestDomain" label="处理域名" width="200" />

            <el-table-column prop="traceId" label="调用链追踪ID" />

          </el-table>

          

 <!-- 数据库表 -->

          <div class="pagination mt-4">

            <div class="pagination-info">

              共 {{ tracingTotal }}  条

            </div>

            <el-pagination

              background

              layout="prev, pager, next, jumper"

              :total="tracingTotal"

              :page-size="tracingPageSize"

              :current-page="tracingCurrentPage"

              @current-change="handleTracingPageChange"

            />

          </div>

        </div>

      </div>

    </div>

    

 <!-- 抽屉-->

    <el-drawer

      v-model="tracingDrawerVisible"

      title="调用链详情"

      direction="rtl"

      size="50%"

    >

      <div class="tracing-drawer">

        <h3>调用链详情</h3>

        <el-descriptions :column="1" border>

          <el-descriptions-item label="开始时间">{{ selectedTracing.startTime }}</el-descriptions-item>

          <el-descriptions-item label="客户端服务">{{ selectedTracing.client }}</el-descriptions-item>

          <el-descriptions-item label="服务端列表">{{ selectedTracing.server }}</el-descriptions-item>

          <el-descriptions-item label="请求类型">{{ selectedTracing.requestType }}</el-descriptions-item>

          <el-descriptions-item label="处理域名">{{ selectedTracing.requestDomain }}</el-descriptions-item>

          <el-descriptions-item label="调用链追踪ID">{{ selectedTracing.traceId }}</el-descriptions-item>

          <el-descriptions-item label="响应状态">{{ selectedTracing.status }}</el-descriptions-item>

          <el-descriptions-item label="分组聚合详情">{{ selectedTracing.responseTime }}</el-descriptions-item>

        </el-descriptions>

        <div class="mt-4">

          <h4>调用链路信息</h4>

          <div class="call-chain-topology">

            <div class="call-chain-node">

              <div class="node-content">

                <div class="node-label">{{ selectedTracing.client }}</div>

              </div>

            </div>

            <div class="call-chain-arrow">→</div>

            <div class="call-chain-node">

              <div class="node-content">

                <div class="node-label">{{ selectedTracing.server }}</div>

              </div>

            </div>

            <div class="call-chain-arrow">→</div>

            <div class="call-chain-node">

              <div class="node-content">

                <div class="node-label">backend-service</div>

              </div>

            </div>

            <div class="call-chain-arrow">→</div>

            <div class="call-chain-node">

              <div class="node-content">

                <div class="node-label">database</div>

              </div>

            </div>

          </div>

        </div>

        <div class="mt-4">

          <h4>关键指标</h4>

          <div class="drawer-charts">

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

import { ref, onMounted } from 'vue'

import { ArrowDown } from '@element-plus/icons-vue'

import { api } from '../utils/api'



// 调用链错误列表

const tracingForm = ref({

  snapshot: '15m',

  search: ''

})



const tracingActionsForm = ref({})



// 信号源选择

const selectedSignalSources = ref(['eBPF', 'OTel'])



// 响应状态选择

const selectedStatuses = ref(['正常', '服务端错误率'])



// 应用协议选择

const selectedProtocols = ref(['HTTP', 'gRPC'])



// 区域查询

const regionQuery = ref('全部')



// 图表数据库流

const trendData = ref([])

const responseTimeData = ref([])



// 应用资源分析加载数据

const tracingData = ref([])

const loading = ref(false)

const error = ref('')



// 数据库表详情

const tracingPageSize = ref(10)

const tracingCurrentPage = ref(1)

const tracingTotal = ref(50)



// 右侧抽屉弹窗

const tracingDrawerVisible = ref(false)

const selectedTracing = ref({

  startTime: '',

  client: '',

  server: '',

  requestType: '',

  requestDomain: '',

  traceId: '',

  status: '',

  responseTime: ''

})



// 加载数据

const loadTracingData = async () => {

  loading.value = true

  error.value = ''

  try {

    const response = await api.getTracing({

      timeRange: tracingForm.value.snapshot,

      serviceName: tracingForm.value.search || undefined

    })

    if (response.data && response.data.traces) {

      tracingData.value = response.data.traces.map((trace: any) => ({

        startTime: new Date(trace.startTime || Date.now()).toLocaleString('zh-CN'),

        client: trace.client || trace.serviceName || 'unknown',

        server: trace.server || 'unknown',

        requestType: trace.requestType || 'POST',

        requestDomain: trace.requestDomain || 'unknown',

        traceId: trace.traceId || '--',

        status: trace.status === 'failed' ? '服务端错误率' : '正常',

        responseTime: trace.duration ? `${trace.duration} ms` : '0 ms'

      }))

      tracingTotal.value = response.data.traces.length

    }

  } catch (err) {

 // console.error('加载数据失败:', err)

    error.value = '加载数据库失败，请稍后重试'

 // 使用模拟数据库作为 fallback

    tracingData.value = [

      {

        startTime: '05-15 11:26:03.997',

        client: 'web-shop-c646fbf0f-m5th',

        server: 'otel-agent-9v6t6',

        requestType: 'POST',

        requestDomain: 'otel-agent.open-telemetry:11888',

        traceId: '调用链追踪ID',

        status: '正常',

        responseTime: '12.5 ms'

      },

      {

        startTime: '05-15 11:26:03.997',

        client: 'web-shop-c646fbf0f-m5th',

        server: 'otel-agent-9v6t6',

        requestType: 'POST',

        requestDomain: 'otel-agent.open-telemetry:11888',

        traceId: '--',

        status: '正常',

        responseTime: '10.3 ms'

      },

      {

        startTime: '05-15 11:26:03.997',

        client: 'web-shop-c646fbf0f-m5th',

        server: 'otel-agent-9v6t6',

        requestType: 'POST',

        requestDomain: 'otel-agent.open-telemetry:11888',

        traceId: 'skywalking',

        status: '正常',

        responseTime: '8.7 ms'

      }

    ]

  } finally {

    loading.value = false

  }

}



// 搜索调用延迟分析
const searchTracing = () => {

  loadTracingData()

}



// 保存搜索条件

const saveSearch = () => {

 // 模拟保存功能

}



// 显示数据库字段
const showDatabaseFields = () => {

 // 模拟显示功能

}



// 切换策略显示

const switchNameDisplay = () => {

 // 模拟切换功能

}



// 刷新调用延迟分析
const refreshTracing = () => {

  loadTracingData()

}



// 导出调用延迟分析
const exportTracing = () => {

 // 模拟导出功能

}



// 切换数据源功能
const toggleColumn = (column: string) => {

 // 模拟切换功能

}



// 处理调用链路点击

const handleTracingRowClick = (row: any) => {

  selectedTracing.value = row

  tracingDrawerVisible.value = true

  }



// 调用链路展示数据库切换
const handleTracingPageChange = (page: number) => {

  tracingCurrentPage.value = page

  }



// 页面加载时初始化数据库

onMounted(() => {
  trendData.value = generateMockData(80, 20, 60)
  responseTimeData.value = generateMockData(90, 10, 20)

  loadTracingData()

})

</script>



<style scoped>

.app-tracing-content {

  padding: 20px;

}



.tracing-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.tracing-search {

  flex: 1;

}



.tracing-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.tracing-content {

  display: flex;

  gap: 20px;

}



.tracing-sidebar {

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



.tracing-main {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.tracing-chart {

  margin-bottom: 30px;

}



.chart-header h3 {

  margin: 0;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.tracing-chart-content {

  height: 200px;

}



.tracing-bar {

  background-color: #409eff;

  border-radius: 2px 2px 0 0;

}



.tracing-list {

  background-color: #f5f7fa;

  border-radius: 4px;

  padding: 15px;

}



.table-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 15px;

}



.table-header h3 {

  margin: 0;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.table-actions {

  display: flex;

  gap: 10px;

}



.tracing-list .el-table {

  margin-bottom: 20px;

}



.tracing-list .el-table th {

  background-color: #f5f7fa;

}



.tracing-list .el-table td {

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



.tracing-drawer {

  padding: 20px;

}



.tracing-drawer h3 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.tracing-drawer h4 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.tracing-drawer h5 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 12px;

  font-weight: bold;

  color: #303133;

}



.call-chain-topology {

  display: flex;

  align-items: center;

  gap: 10px;

  padding: 20px;

  background-color: #f5f7fa;

  border-radius: 4px;

  margin-bottom: 20px;

}



.call-chain-node {

  flex: 1;

  text-align: center;

}



.node-content {

  padding: 10px;

  background-color: white;

  border: 1px solid #409eff;

  border-radius: 4px;

  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);

}



.node-label {

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.call-chain-arrow {

  font-size: 20px;

  color: #409eff;

  font-weight: bold;

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

  .tracing-content {

    flex-direction: column;

  }

  

  .tracing-sidebar {

    width: 100%;

  }

  

  .call-chain-topology {

    flex-direction: column;

    gap: 20px;

  }

  

  .call-chain-arrow {

    transform: rotate(90deg);

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