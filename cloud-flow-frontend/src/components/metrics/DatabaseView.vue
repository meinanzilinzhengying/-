<template>
  <div class="metrics-view-content">
    <!-- 顶部控制 -->
    <div class="metrics-view-header">
      <div class="time-range-selector">
        <el-select v-model="metricsTimeRange" placeholder="最近15分钟" style="margin-right: 10px;">
          <el-option label="最近15分钟" value="5m" />
          <el-option label="最近15分钟" value="15m" />
          <el-option label="最近30分钟" value="30m" />
          <el-option label="最近1小时" value="1h" />
          <el-option label="最近6小时" value="6h" />
          <el-option label="最近12小时" value="12h" />
          <el-option label="最近24小时" value="24h" />
          <el-option label="最近7天" value="7d" />
        </el-select>
        <el-button type="primary" @click="refreshMetrics">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
        <el-button @click="autoRefresh = !autoRefresh">
          自动 ({{ autoRefresh ? '开' : '关' }})
        </el-button>
      </div>
    </div>

    <!-- 指标告警管理 -->
    <div class="metrics-config">
      <!-- 数据库表A -->
      <div class="metrics-table">
        <div class="table-header">
          <el-collapse v-model="tableACollapsed">
            <el-collapse-item title="A 数据库" name="1">
              <div class="table-controls">
                <el-form :inline="true" :model="tableAForm" class="demo-form-inline">
                  <el-form-item label="应用">
                    <el-select v-model="tableAForm.application" placeholder="选择应用" style="width: 150px;">
                      <el-option label="应用1" value="app1" />
                      <el-option label="应用2" value="app2" />
                      <el-option label="应用3" value="app3" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="聚合间隔">
                    <el-select v-model="tableAForm.interval" placeholder="分钟" style="width: 150px;">
                      <el-option label="秒级" value="second" />
                      <el-option label="分钟" value="minute" />
                      <el-option label="小时" value="hour" />
                      <el-option label="天级" value="day" />
                    </el-select>
                  </el-form-item>
                  <el-form-item>
                    <el-button type="primary" @click="applyTableA">应用</el-button>
                  </el-form-item>
                  <el-form-item>
                    <el-button type="danger" @click="clearTableA">
                      <el-icon><Delete /></el-icon>
                    </el-button>
                  </el-form-item>
                </el-form>

                <!-- 搜索条件 -->
                <div class="search-condition">
                  <el-input v-model="tableASearch" placeholder="输入搜索条件" style="width: 500px;" />
                  <el-select v-model="tableAGroupBy" placeholder="分组" style="margin-left: 10px; width: 120px;">
                    <el-option label="无" value="none" />
                    <el-option label="应用" value="app" />
                    <el-option label="主机" value="host" />
                  </el-select>
                  <el-select v-model="tableASubGroupBy" placeholder="子分组" style="margin-left: 10px; width: 120px;">
                    <el-option label="无" value="none" />
                    <el-option label="应用" value="app" />
                    <el-option label="主机" value="host" />
                  </el-select>
                </div>
              </div>
            </el-collapse-item>
          </el-collapse>
        </div>

        <!-- 指标列表 -->
        <div class="metrics-list">
          <div v-for="(metric, index) in tableAMetrics" :key="index" class="metric-item">
            <el-form :inline="true" :model="metric" class="demo-form-inline">
              <el-form-item>
                <el-select v-model="metric.name" placeholder="选择指标" style="width: 120px;">
                  <el-option label="请求" value="request" />
                  <el-option label="响应" value="response" />
                  <el-option label="错误" value="error" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-select v-model="metric.aggregation" placeholder="选择聚合方式" style="width: 120px;">
                  <el-option label="Avg" value="avg" />
                  <el-option label="Sum" value="sum" />
                  <el-option label="Max" value="max" />
                  <el-option label="Min" value="min" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button @click="editMetric(index, 'A')">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-form-item>
              <el-form-item>
                <el-switch v-model="metric.enabled" style="margin-right: 10px;" />
              </el-form-item>
              <el-form-item>
                <el-button type="danger" @click="removeMetric(index, 'A')">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-form-item>
            </el-form>
          </div>

          <!-- 添加指标按钮 -->
          <div class="add-metric">
            <el-button type="primary" @click="addMetric('A')">
              <el-icon><Plus /></el-icon> 添加指标
            </el-button>
          </div>
        </div>
      </div>

      <!-- 数据库表B -->
      <div class="metrics-table">
        <div class="table-header">
          <el-collapse v-model="tableBCollapsed">
            <el-collapse-item title="B 数据库" name="1">
              <div class="table-controls">
                <el-form :inline="true" :model="tableBForm" class="demo-form-inline">
                  <el-form-item label="应用">
                    <el-select v-model="tableBForm.application" placeholder="选择应用" style="width: 150px;">
                      <el-option label="应用1" value="app1" />
                      <el-option label="应用2" value="app2" />
                      <el-option label="应用3" value="app3" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="日志类型">
                    <el-select v-model="tableBForm.logType" placeholder="端侧日志" style="width: 150px;">
                      <el-option label="端侧日志" value="client" />
                      <el-option label="服务端日" value="server" />
                      <el-option label="线程日志" value="system" />
                    </el-select>
                  </el-form-item>
                  <el-form-item>
                    <el-button type="primary" @click="applyTableB">应用</el-button>
                  </el-form-item>
                  <el-form-item>
                    <el-button type="danger" @click="clearTableB">
                      <el-icon><Delete /></el-icon>
                    </el-button>
                  </el-form-item>
                </el-form>

                <!-- 搜索条件 -->
                <div class="search-condition">
                  <el-input v-model="tableBSearch" placeholder="输入搜索条件" style="width: 500px;" />
                  <el-select v-model="tableBGroupBy" placeholder="分组" style="margin-left: 10px; width: 120px;">
                    <el-option label="无" value="none" />
                    <el-option label="应用" value="app" />
                    <el-option label="主机" value="host" />
                  </el-select>
                  <el-select v-model="tableBSubGroupBy" placeholder="子分组" style="margin-left: 10px; width: 120px;">
                    <el-option label="无" value="none" />
                    <el-option label="应用" value="app" />
                    <el-option label="主机" value="host" />
                  </el-select>
                </div>
              </div>
            </el-collapse-item>
          </el-collapse>
        </div>

        <!-- 指标列表 -->
        <div class="metrics-list">
          <div v-for="(metric, index) in tableBMetrics" :key="index" class="metric-item">
            <el-form :inline="true" :model="metric" class="demo-form-inline">
              <el-form-item>
                <el-select v-model="metric.name" placeholder="选择指标" style="width: 120px;">
                  <el-option label="分组聚合详情" value="response_time" />
                  <el-option label="错误" value="error_rate" />
                  <el-option label="吞吐" value="throughput" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-select v-model="metric.aggregation" placeholder="选择聚合方式" style="width: 120px;">
                  <el-option label="Avg" value="avg" />
                  <el-option label="Sum" value="sum" />
                  <el-option label="Max" value="max" />
                  <el-option label="Min" value="min" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button @click="editMetric(index, 'B')">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-form-item>
              <el-form-item>
                <el-switch v-model="metric.enabled" style="margin-right: 10px;" />
              </el-form-item>
              <el-form-item>
                <el-button type="danger" @click="removeMetric(index, 'B')">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-form-item>
            </el-form>
          </div>

          <!-- 添加指标按钮 -->
          <div class="add-metric">
            <el-button type="primary" @click="addMetric('B')">
              <el-icon><Plus /></el-icon> 添加指标
            </el-button>
          </div>
        </div>
      </div>

      <!-- 添加查询按钮 -->
      <div class="add-query">
        <el-button type="success" @click="addQuery">
          <el-icon><Plus /></el-icon> 添加查询
        </el-button>
      </div>
    </div>

    <!-- 图表展示和布局管理 -->
    <div class="metrics-charts">
      <el-row :gutter="20">
        <el-col :span="8">
          <el-card class="mb-4">
            <template #header>
              <div class="chart-header">
                <h3>Avg(请求)</h3>
              </div>
            </template>
            <div class="chart-content">
              <div class="mock-chart">
                <div class="chart-bars">
                  <div v-for="(height, index) in cpuChartData" :key="index" class="chart-bar cpu-bar" :style="{ height: height + '%' }"></div>
                </div>
                <div class="chart-x-axis">
                  <div v-for="i in 6" :key="i" class="x-axis-label">{{ 17 + i }}:47</div>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>

        <el-col :span="8">
          <el-card class="mb-4">
            <template #header>
              <div class="chart-header">
                <h3>Avg(响应)</h3>
              </div>
            </template>
            <div class="chart-content">
              <div class="mock-chart">
                <div class="chart-bars">
                  <div v-for="(height, index) in memChartData" :key="index" class="chart-bar mem-bar" :style="{ height: height + '%' }"></div>
                </div>
                <div class="chart-x-axis">
                  <div v-for="i in 6" :key="i" class="x-axis-label">{{ 17 + i }}:47</div>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>

        <el-col :span="8">
          <el-card class="mb-4">
            <template #header>
              <div class="chart-header">
                <h3>Avg(分组聚合详情)</h3>
              </div>
            </template>
            <div class="chart-content">
              <div class="mock-chart">
                <div class="chart-bars">
                  <div v-for="(height, index) in netInChartData" :key="index" class="chart-bar net-in-bar" :style="{ height: height + '%' }"></div>
                </div>
                <div class="chart-x-axis">
                  <div v-for="i in 6" :key="i" class="x-axis-label">{{ 17 + i }}:47</div>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Refresh, Delete, Edit, Plus } from '@element-plus/icons-vue'
import { useMockData } from '@/composables/useMockData'

const { generateMockData } = useMockData()

// 时间范围选择
const metricsTimeRange = ref('5m')

// 自动刷新
const autoRefresh = ref(false)

// 数据库表A折叠面板
const tableACollapsed = ref(false)

// 数据库表B折叠面板
const tableBCollapsed = ref(false)

// 数据库表A表单
const tableAForm = ref({
  application: '',
  interval: 'minute'
})

// 数据库表B表单
const tableBForm = ref({
  application: '',
  logType: 'client'
})

// 数据库表A搜索条件
const tableASearch = ref('')
const tableAGroupBy = ref('none')
const tableASubGroupBy = ref('none')

// 数据库表B搜索条件
const tableBSearch = ref('')
const tableBGroupBy = ref('none')
const tableBSubGroupBy = ref('none')

// 数据库表A指标
const tableAMetrics = ref([
  { name: 'request', aggregation: 'avg', enabled: true },
  { name: 'response', aggregation: 'avg', enabled: true }
])

// 数据库表B指标
const tableBMetrics = ref([
  { name: 'response_time', aggregation: 'avg', enabled: true }
])

// 图表数据
const cpuChartData = ref<number[]>([])
const memChartData = ref<number[]>([])
const netInChartData = ref<number[]>([])

// 刷新指标数据
const refreshMetrics = () => {
  // 使用模拟数据
}

// 应用数据库表A配置
const applyTableA = () => {
  // 应用配置
}

// 清除数据库表A配置
const clearTableA = () => {
  tableAForm.value = {
    application: '',
    interval: 'minute'
  }
  tableASearch.value = ''
  tableAGroupBy.value = 'none'
  tableASubGroupBy.value = 'none'
}

// 应用数据库表B配置
const applyTableB = () => {
  // 应用配置
}

// 清除数据库表B配置
const clearTableB = () => {
  tableBForm.value = {
    application: '',
    logType: 'client'
  }
  tableBSearch.value = ''
  tableBGroupBy.value = 'none'
  tableBSubGroupBy.value = 'none'
}

// 添加指标
const addMetric = (table: string) => {
  if (table === 'A') {
    tableAMetrics.value.push({
      name: 'request',
      aggregation: 'avg',
      enabled: true
    })
  } else if (table === 'B') {
    tableBMetrics.value.push({
      name: 'response_time',
      aggregation: 'avg',
      enabled: true
    })
  }
}

// 编辑指标
const editMetric = (index: number, table: string) => {
  // 编辑指标
}

// 移除指标
const removeMetric = (index: number, table: string) => {
  if (table === 'A') {
    tableAMetrics.value.splice(index, 1)
  } else if (table === 'B') {
    tableBMetrics.value.splice(index, 1)
  }
}

// 添加查询
const addQuery = () => {
  // 模拟添加查询
}

// 初始化
onMounted(() => {
  cpuChartData.value = generateMockData(150, 50, 30)
  memChartData.value = generateMockData(120, 30, 30)
  netInChartData.value = generateMockData(200, 20, 30)
})
</script>

<style scoped>
.metrics-view-content {
  padding: 20px;
}

.metrics-view-header {
  margin-bottom: 20px;
  padding: 15px;
  background-color: #f5f7fa;
  border-radius: 4px;
}

.time-range-selector {
  display: flex;
  align-items: center;
}

.metrics-config {
  margin-bottom: 30px;
}

.metrics-table {
  margin-bottom: 20px;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  background-color: white;
}

.table-header {
  border-bottom: 1px solid #ebeef5;
}

.table-controls {
  padding: 15px;
}

.search-condition {
  margin-top: 15px;
  display: flex;
  align-items: center;
}

.metrics-list {
  padding: 15px;
}

.metric-item {
  margin-bottom: 10px;
  padding: 10px;
  background-color: #f9f9f9;
  border-radius: 4px;
}

.add-metric {
  margin-top: 15px;
  text-align: right;
}

.add-query {
  margin-top: 20px;
  text-align: right;
}

.metrics-charts {
  margin-top: 30px;
}

.chart-header h3 {
  margin: 0 0 15px 0;
  font-size: 16px;
  font-weight: bold;
}

.mock-chart {
  background-color: #f5f7fa;
  padding: 20px;
  border-radius: 4px;
  height: 200px;
  display: flex;
  flex-direction: column;
}

.chart-bars {
  flex: 1;
  display: flex;
  align-items: flex-end;
  gap: 3px;
  margin-bottom: 10px;
}

.chart-bar {
  flex: 1;
  border-radius: 4px 4px 0 0;
  min-height: 10px;
}

.cpu-bar {
  background-color: #409eff;
}

.mem-bar {
  background-color: #67c23a;
}

.net-in-bar {
  background-color: #e6a23c;
}

.chart-x-axis {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #909399;
}
</style>
