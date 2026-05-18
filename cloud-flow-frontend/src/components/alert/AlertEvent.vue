<template>
  <el-card class="mb-4">
    <template #header>
      <div class="card-header">
        <h2>告警事件</h2>
        <div class="header-actions">
          <el-button @click="refreshAlertEvents">
            <el-icon><Refresh /></el-icon> 刷新
          </el-button>
        </div>
      </div>
    </template>

    <!-- 时间范围选择 -->
    <div class="time-range-select mb-4">
      <el-select v-model="timeRange" placeholder="最近一天" style="margin-right: 10px;">
        <el-option label="最近一天" value="1d" />
        <el-option label="最近一周" value="7d" />
        <el-option label="最近一个月" value="30d" />
        <el-option label="自定义" value="custom" />
      </el-select>
      <el-date-picker
        v-if="timeRange === 'custom'"
        v-model="dateRange"
        type="daterange"
        range-separator="至"
        start-placeholder="开始时间"
        end-placeholder="结束时间"
        format="YYYY-MM-DD HH:mm:ss"
        value-format="YYYY-MM-DD HH:mm:ss"
        style="margin-right: 10px;"
      />
      <el-button type="primary" @click="handleSearch">搜索</el-button>
    </div>

    <!-- 过滤面板 -->
    <div class="filter-panel mb-4">
      <div class="filter-section">
        <h3>过滤面板</h3>
        <div class="filter-content">
          <!-- 策略等级 -->
          <div class="filter-item">
            <h4>策略等级</h4>
            <el-checkbox-group v-model="filterForm.strategyLevel">
              <el-checkbox label="高">高</el-checkbox>
              <el-checkbox label="中">中</el-checkbox>
              <el-checkbox label="低">低</el-checkbox>
            </el-checkbox-group>
          </div>

          <!-- 事件等级 -->
          <div class="filter-item">
            <h4>事件等级</h4>
            <el-checkbox-group v-model="filterForm.eventLevel">
              <el-checkbox label="致命">致命</el-checkbox>
              <el-checkbox label="错误">错误</el-checkbox>
              <el-checkbox label="警告">警告</el-checkbox>
              <el-checkbox label="恢复">恢复</el-checkbox>
              <el-checkbox label="已通知">已通知</el-checkbox>
              <el-checkbox label="未确认">未确认</el-checkbox>
            </el-checkbox-group>
          </div>

          <!-- 监控对象 -->
          <div class="filter-item">
            <h4>监控对象</h4>
            <el-checkbox-group v-model="filterForm.monitorObject">
              <el-checkbox label="指标">指标</el-checkbox>
              <el-checkbox label="系统">系统</el-checkbox>
            </el-checkbox-group>
          </div>

          <!-- 区域 -->
          <div class="filter-item">
            <h4>区域</h4>
            <el-checkbox-group v-model="filterForm.region">
              <el-checkbox label="华北-北京">华北-北京</el-checkbox>
              <el-checkbox label="华中-上海">华中-上海</el-checkbox>
              <el-checkbox label="华南-广州">华南-广州</el-checkbox>
            </el-checkbox-group>
          </div>

          <!-- 创建标签 -->
          <div class="filter-item">
            <h4>创建标签</h4>
            <el-checkbox-group v-model="filterForm.creator">
              <el-checkbox label="admin">admin</el-checkbox>
              <el-checkbox label="user1">user1</el-checkbox>
              <el-checkbox label="user2">user2</el-checkbox>
            </el-checkbox-group>
          </div>
        </div>
      </div>
    </div>

    <!-- 搜索框 -->
    <div class="search-box mb-4">
      <el-input
        v-model="searchKeyword"
        placeholder="输入查询条件，如 policy_id = 145 等..."
        style="width: 400px;"
      >
        <template #append>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
        </template>
      </el-input>
    </div>

    <!-- 图表容器 -->
    <div class="chart-container mb-4">
      <div class="chart-header">
        <h3>异常分析</h3>
        <div class="chart-controls">
          <el-select v-model="chartType" placeholder="选择维度" style="margin-right: 10px;">
            <el-option label="按小时" value="hour" />
            <el-option label="按天" value="day" />
            <el-option label="按周" value="week" />
          </el-select>
          <el-select v-model="chartRange" placeholder="查看范围管理" style="margin-right: 10px;">
            <el-option label="全部" value="all" />
            <el-option label="华北-北京" value="beijing" />
            <el-option label="华中-上海" value="shanghai" />
            <el-option label="华南-广州" value="guangzhou" />
          </el-select>
        </div>
      </div>
      <div class="chart-content">
        <div class="mock-chart">
          <div class="chart-title">告警事件趋势</div>
          <div class="chart-bars">
            <div v-for="(height, index) in chartData" :key="index" class="chart-bar" :style="{ height: height + '%' }"></div>
          </div>
          <div class="chart-x-axis">
            <div v-for="i in 6" :key="i" class="x-axis-label">{{ i * 2 }}:00</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 告警事件列表 -->
    <div class="event-table">
      <div class="table-header">
        <div class="table-title">事件详情表（共27条数据库）</div>
        <div class="table-controls">
          <el-button type="primary" @click="handleExport">导出</el-button>
          <el-button @click="handleToggleColumns">列选择</el-button>
        </div>
      </div>

      <el-table
        :data="alertEvents"
        style="width: 100%"
        @row-dblclick="handleViewDetail"
      >
        <el-table-column prop="startTime" label="开始时间" width="180" sortable />
        <el-table-column prop="name" label="事件名称" min-width="200" />
        <el-table-column prop="level" label="事件等级" width="100">
          <template #default="scope">
            <el-tag :type="getLevelType(scope.row.level)">{{ scope.row.level }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="monitorObject" label="监控对象" width="120" />
        <el-table-column prop="eventInfo" label="事件信息" min-width="300" />
        <el-table-column prop="creator" label="创建标签" width="100" />
      </el-table>

      <!-- 分页 -->
      <div class="pagination mt-4">
        <div class="pagination-info">
          共 {{ alertEventTotal }} 条
        </div>
        <el-pagination
          background
          layout="prev, pager, next, jumper"
          :total="alertEventTotal"
          :page-size="pagination.pageSize"
          :current-page="pagination.currentPage"
          @current-change="handleCurrentChange"
        />
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'

// 定义事件
const emit = defineEmits<{
  (e: 'view-detail', event: any): void
  (e: 'export'): void
  (e: 'toggle-columns'): void
}>()

// 生成模拟数据
const generateMockData = (max: number, min: number, count: number = 30) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

// 时间范围
const timeRange = ref('1d')
const dateRange = ref([])

// 搜索关键词
const searchKeyword = ref('')

// 图表类型和范围
const chartType = ref('hour')
const chartRange = ref('all')

// 图表数据
const chartData = ref<number[]>([])

onMounted(() => {
  chartData.value = generateMockData(80, 20, 24)
})

// 过滤表单
const filterForm = ref({
  strategyLevel: [],
  eventLevel: [],
  monitorObject: [],
  region: [],
  creator: []
})

// 分页
const pagination = reactive({
  pageSize: 10,
  currentPage: 1
})

// 告警事件数据
const alertEvents = ref([
  { id: '1', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:53:00', endTime: '2023-05-18 10:53:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '2', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:52:00', endTime: '2023-05-18 10:52:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '3', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:51:00', endTime: '2023-05-18 10:51:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '4', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:49:00', endTime: '2023-05-18 10:49:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '5', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:48:00', endTime: '2023-05-18 10:48:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '6', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:47:00', endTime: '2023-05-18 10:47:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '7', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:46:00', endTime: '2023-05-18 10:46:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '8', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:45:00', endTime: '2023-05-18 10:45:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '9', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:44:00', endTime: '2023-05-18 10:44:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' },
  { id: '10', name: '数据库节点连接异常 (ingester.receiver.event)', level: '警告', monitorObject: '系统', eventInfo: 'queries: region=华中-上海, tag.host=警告: drop_packets(16) > 1. 触发阈值 1.000000', creator: 'admin', startTime: '2023-05-18 10:43:00', endTime: '2023-05-18 10:43:00', region: '华中-上海', triggerCondition: 'drop_packets(16) > 1', suggestion: '检查数据库节点网络连接和接收能力' }
])

const alertEventTotal = ref(227)

// 搜索处理
const handleSearch = () => {
  ElMessage.info('功能开发中...')
}

// 分页处理
const handleCurrentChange = (page: number) => {
  pagination.currentPage = page
}

// 刷新告警事件
const refreshAlertEvents = () => {
  ElMessage.info('功能开发中...')
}

// 导出事件
const handleExport = () => {
  emit('export')
}

// 列选择
const handleToggleColumns = () => {
  emit('toggle-columns')
}

// 查看详情
const handleViewDetail = (event: any) => {
  emit('view-detail', event)
}

// 获取等级类型
const getLevelType = (level: string) => {
  switch (level) {
    case '高':
    case '致命':
      return 'danger'
    case '中':
    case '错误':
      return 'warning'
    case '低':
    case '警告':
      return 'info'
    default:
      return ''
  }
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.time-range-select {
  display: flex;
  align-items: center;
  margin-bottom: 20px;
}

.filter-panel {
  background-color: #f5f7fa;
  padding: 15px;
  border-radius: 4px;
  margin-bottom: 20px;
}

.filter-section h3 {
  margin-top: 0;
  margin-bottom: 15px;
  font-size: 16px;
  font-weight: bold;
}

.filter-content {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 20px;
}

.filter-item h4 {
  margin-top: 0;
  margin-bottom: 10px;
  font-size: 14px;
  font-weight: bold;
}

.filter-item .el-checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.search-box {
  margin-bottom: 20px;
}

.chart-container {
  background-color: #f5f7fa;
  padding: 15px;
  border-radius: 4px;
  margin-bottom: 20px;
}

.chart-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.chart-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: bold;
}

.chart-controls {
  display: flex;
  align-items: center;
}

.mock-chart {
  background-color: white;
  padding: 20px;
  border-radius: 4px;
  height: 200px;
  display: flex;
  flex-direction: column;
}

.chart-title {
  margin-bottom: 10px;
  font-size: 14px;
  font-weight: bold;
}

.chart-bars {
  flex: 1;
  display: flex;
  align-items: flex-end;
  gap: 5px;
  margin-bottom: 10px;
}

.chart-bar {
  flex: 1;
  background-color: #409eff;
  border-radius: 4px 4px 0 0;
  min-height: 20px;
}

.chart-x-axis {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #909399;
}

.event-table {
  background-color: white;
  border-radius: 4px;
  padding: 15px;
}

.table-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.table-title {
  font-size: 16px;
  font-weight: bold;
}

.table-controls {
  display: flex;
  gap: 10px;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pagination-info {
  color: #909399;
  font-size: 14px;
}
</style>
