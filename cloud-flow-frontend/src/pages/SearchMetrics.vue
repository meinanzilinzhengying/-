<template>
  <div class="search-metrics">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">管理后台</router-link></el-breadcrumb-item>
        <el-breadcrumb-item><router-link to="/search">数据库搜索</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>指标搜索</el-breadcrumb-item>
      </el-breadcrumb>
      <h2>指标搜索</h2>
    </div>

    <!-- 搜索快照 -->
    <div class="snapshot-section">
      <SearchSnapshot />
    </div>

    <!-- 指标搜索框 -->
    <div class="metrics-search-section">
      <MetricsSearch />
    </div>

    <!-- 指标概览统计 -->
    <div class="metrics-overview">
      <el-row :gutter="16">
        <el-col :span="6" v-for="stat in metricsStats" :key="stat.label">
          <div class="stat-card">
            <div class="stat-label">{{ stat.label }}</div>
            <div class="stat-value" :style="{ color: stat.color }">{{ stat.value }}</div>
          </div>
        </el-col>
      </el-row>
    </div>

    <!-- 指标图表展示 -->
    <div class="chart-section">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>指标图表</span>
            <div class="chart-actions">
              <el-radio-group v-model="chartType" size="small">
                <el-radio-button label="line">折线图</el-radio-button>
                <el-radio-button label="bar">柱状图</el-radio-button>
                <el-radio-button label="area">面积图</el-radio-button>
              </el-radio-group>
              <el-button size="small" @click="refreshMetrics">
                <el-icon><Refresh /></el-icon> 刷新
              </el-button>
            </div>
          </div>
        </template>

        <!-- 模拟图表区域 -->
        <div class="chart-container">
          <div class="chart-legend">
            <span class="legend-item"><span class="legend-dot" style="background-color: #409eff;"></span> CPU 使用率</span>
            <span class="legend-item"><span class="legend-dot" style="background-color: #67c23a;"></span> 内存使用率</span>
            <span class="legend-item"><span class="legend-dot" style="background-color: #e6a23c;"></span> 网络吞吐</span>
          </div>
          <div class="mock-chart">
            <div class="chart-bars">
              <div v-for="i in 60" :key="i" class="chart-group">
                <div class="chart-bar cpu-bar" :style="{ height: cpuData[i-1] + '%' }"></div>
                <div class="chart-bar mem-bar" :style="{ height: memData[i-1] + '%' }"></div>
                <div class="chart-bar net-bar" :style="{ height: netData[i-1] + '%' }"></div>
              </div>
            </div>
            <div class="chart-x-axis">
              <div v-for="i in 6" :key="i" class="x-axis-label">{{ 10 + i }}:00</div>
            </div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 指标数据表格 -->
    <div class="table-section">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>指标数据明细</span>
            <el-button size="small" @click="exportMetrics">
              <el-icon><Download /></el-icon> 导出
            </el-button>
          </div>
        </template>
        <el-table :data="metricsTableData" style="width: 100%" stripe>
          <el-table-column prop="metricName" label="指标名称" min-width="180" />
          <el-table-column prop="host" label="主机" width="150" />
          <el-table-column prop="currentValue" label="当前值" width="120" align="center">
            <template #default="scope">
              <span :class="getValueClass(scope.row.currentValue, scope.row.threshold)">
                {{ scope.row.currentValue }}{{ scope.row.unit }}
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="avgValue" label="平均值" width="120" align="center">
            <template #default="scope">
              {{ scope.row.avgValue }}{{ scope.row.unit }}
            </template>
          </el-table-column>
          <el-table-column prop="maxValue" label="最大值" width="120" align="center">
            <template #default="scope">
              {{ scope.row.maxValue }}{{ scope.row.unit }}
            </template>
          </el-table-column>
          <el-table-column prop="threshold" label="阈值" width="120" align="center">
            <template #default="scope">
              {{ scope.row.threshold }}{{ scope.row.unit }}
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="80" align="center">
            <template #default="scope">
              <el-tag :type="scope.row.status === '正常' ? 'success' : scope.row.status === '告警' ? 'warning' : 'danger'" size="small">
                {{ scope.row.status }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="updateTime" label="更新时间" width="170" />
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
  </div>
</template>

<script setup lang="ts">
import SearchSnapshot from '../components/SearchSnapshot.vue'
import MetricsSearch from '../components/MetricsSearch.vue'
import { Refresh, Download } from '@element-plus/icons-vue'
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'

const chartType = ref('line')

// 概览统计
const metricsStats = ref([
  { label: '监控指标总数', value: '1,286', color: '#409eff' },
  { label: '告警指标', value: '23', color: '#f56c6c' },
  { label: '数据采集点/分钟', value: '52,400', color: '#67c23a' },
  { label: '存储使用率', value: '67.3%', color: '#e6a23c' }
])

// 模拟图表数据
const generateMockData = (max: number, min: number, count: number = 60) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

const cpuData = ref<number[]>([])
const memData = ref<number[]>([])
const netData = ref<number[]>([])

onMounted(() => {
  cpuData.value = generateMockData(60, 20, 60)
  memData.value = generateMockData(80, 40, 60)
  netData.value = generateMockData(50, 10, 60)
})

// 指标表格数据
const metricsTableData = ref([
  { metricName: 'system.cpu.utilization', host: 'node-01', currentValue: 72.5, avgValue: 65.3, maxValue: 89.2, threshold: 80, unit: '%', status: '告警', updateTime: '2024-01-15 10:30:00' },
  { metricName: 'system.memory.utilization', host: 'node-01', currentValue: 68.2, avgValue: 62.1, maxValue: 75.8, threshold: 85, unit: '%', status: '正常', updateTime: '2024-01-15 10:30:00' },
  { metricName: 'system.network.in.bytes', host: 'node-02', currentValue: 125.6, avgValue: 98.4, maxValue: 256.3, threshold: 200, unit: 'MB/s', status: '正常', updateTime: '2024-01-15 10:30:00' },
  { metricName: 'system.disk.io.utilization', host: 'node-03', currentValue: 92.1, avgValue: 78.5, maxValue: 99.8, threshold: 90, unit: '%', status: '异常', updateTime: '2024-01-15 10:30:00' },
  { metricName: 'system.cpu.utilization', host: 'node-02', currentValue: 45.3, avgValue: 42.1, maxValue: 67.8, threshold: 80, unit: '%', status: '正常', updateTime: '2024-01-15 10:30:00' },
  { metricName: 'system.memory.utilization', host: 'node-03', currentValue: 88.7, avgValue: 82.3, maxValue: 93.1, threshold: 85, unit: '%', status: '异常', updateTime: '2024-01-15 10:30:00' },
  { metricName: 'system.network.out.bytes', host: 'node-01', currentValue: 85.2, avgValue: 72.6, maxValue: 145.9, threshold: 200, unit: 'MB/s', status: '正常', updateTime: '2024-01-15 10:30:00' },
  { metricName: 'process.cpu.utilization', host: 'node-04', currentValue: 55.8, avgValue: 48.2, maxValue: 78.3, threshold: 80, unit: '%', status: '正常', updateTime: '2024-01-15 10:30:00' }
])

const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(1286)

const getValueClass = (current: number, threshold: number) => {
  if (current >= threshold) return 'value-danger'
  if (current >= threshold * 0.8) return 'value-warning'
  return 'value-normal'
}

const handlePageChange = (page: number) => {
  currentPage.value = page
}

const refreshMetrics = () => {
  cpuData.value = generateMockData(60, 20, 60)
  memData.value = generateMockData(80, 40, 60)
  netData.value = generateMockData(50, 10, 60)
  ElMessage.success('指标数据已刷新')
}

const exportMetrics = () => {
  ElMessage.info('导出功能开发中...')
}
</script>

<style scoped>
.search-metrics {
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

.metrics-search-section {
  margin-bottom: 24px;
}

.metrics-overview {
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

.chart-section {
  margin-bottom: 24px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chart-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.chart-container {
  padding: 10px 0;
}

.chart-legend {
  display: flex;
  gap: 24px;
  margin-bottom: 16px;
  justify-content: center;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #606266;
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
}

.mock-chart {
  position: relative;
  width: 100%;
  height: 250px;
  overflow: hidden;
}

.chart-bars {
  display: flex;
  align-items: flex-end;
  height: 80%;
  gap: 1px;
  padding: 0 10px;
}

.chart-group {
  flex: 1;
  display: flex;
  gap: 1px;
  align-items: flex-end;
}

.chart-bar {
  flex: 1;
  min-height: 2px;
  border-radius: 1px 1px 0 0;
}

.cpu-bar { background-color: #409eff; }
.mem-bar { background-color: #67c23a; }
.net-bar { background-color: #e6a23c; }

.chart-x-axis {
  display: flex;
  justify-content: space-between;
  height: 20%;
  padding: 0 10px;
  margin-top: 5px;
}

.x-axis-label {
  font-size: 11px;
  color: #909399;
  text-align: center;
  flex: 1;
}

.table-section {
  flex: 1;
  overflow: auto;
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

.value-normal { color: #67c23a; }
.value-warning { color: #e6a23c; }
.value-danger { color: #f56c6c; font-weight: bold; }
</style>
