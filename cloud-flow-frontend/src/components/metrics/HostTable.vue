<template>
  <div class="host-page">
    <!-- 搜索和筛选-->
    <div class="search-filter mb-4">
      <el-form :inline="true" :model="searchForm" class="demo-form-inline">
        <el-form-item label="时间范围">
          <el-input v-model="searchForm.keyword" placeholder="输入主机名或IP" style="width: 300px;" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 主机列表 -->
    <div class="host-list">
      <el-table
        :data="hosts"
        style="width: 100%"
        @row-click="handleRowClick"
      >
        <el-table-column prop="name" label="主机名称" width="200" />
        <el-table-column prop="ip" label="实例IP" width="150" />
        <el-table-column prop="os" label="操作系统" width="150" />
        <el-table-column prop="cpuUsage" label="CPU使用率" width="180">
          <template #default="scope">
            <div class="progress-container">
              <el-progress
                :percentage="scope.row.cpuUsage"
                :color="getProgressColor(scope.row.cpuUsage)"
                :stroke-width="10"
              />
              <span class="progress-text">{{ scope.row.cpuUsage }}%</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="memUsage" label="MEM使用率" width="180">
          <template #default="scope">
            <div class="progress-container">
              <el-progress
                :percentage="scope.row.memUsage"
                :color="getProgressColor(scope.row.memUsage)"
                :stroke-width="10"
              />
              <span class="progress-text">{{ scope.row.memUsage }}%</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="load" label="系统负载" width="100" />
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="scope">
            <el-button size="small" type="primary" @click="handleRowClick(scope.row)">
              查看详情
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 右滑框查看历史指标 -->
      <el-drawer
        v-model="drawerVisible"
        direction="rtl"
        :title="`${selectedHost?.name} (${selectedHost?.ip})`"
        size="70%"
      >
        <div class="host-detail">
          <div class="detail-header">
            <el-descriptions :column="3">
              <el-descriptions-item label="主机名称">{{ selectedHost?.name }}</el-descriptions-item>
              <el-descriptions-item label="实例IP">{{ selectedHost?.ip }}</el-descriptions-item>
              <el-descriptions-item label="操作系统">{{ selectedHost?.os }}</el-descriptions-item>
              <el-descriptions-item label="CPU使用">{{ selectedHost?.cpuUsage }}%</el-descriptions-item>
              <el-descriptions-item label="MEM使用">{{ selectedHost?.memUsage }}%</el-descriptions-item>
              <el-descriptions-item label="系统负载">{{ selectedHost?.load }}</el-descriptions-item>
            </el-descriptions>
          </div>

          <!-- 时间范围选择 -->
          <div class="time-range-select mb-4">
            <el-select v-model="historyTimeRange" placeholder="最近小时" style="margin-right: 10px;">
              <el-option label="最近1小时" value="1h" />
              <el-option label="最近6小时" value="6h" />
              <el-option label="最近12小时" value="12h" />
              <el-option label="最近24小时" value="24h" />
              <el-option label="最近7天" value="7d" />
            </el-select>
            <el-button type="primary" @click="refreshHistory">
              <el-icon><Refresh /></el-icon> 刷新
            </el-button>
          </div>

          <!-- 历史指标图表 -->
          <div class="history-charts">
            <el-row :gutter="20">
              <el-col :span="12">
                <el-card class="mb-4">
                  <template #header>
                    <div class="chart-header">
                      <h3>CPU使用率</h3>
                    </div>
                  </template>
                  <div class="chart-content">
                    <div class="mock-chart">
                      <div class="chart-bars">
                        <div v-for="i in 30" :key="i" class="chart-bar cpu-bar" :style="{ height: cpuHistoryData[i-1] + '%' }"></div>
                      </div>
                      <div class="chart-x-axis">
                        <div v-for="i in 6" :key="i" class="x-axis-label">{{ 12 + i }}:00</div>
                      </div>
                    </div>
                  </div>
                </el-card>
              </el-col>

              <el-col :span="12">
                <el-card class="mb-4">
                  <template #header>
                    <div class="chart-header">
                      <h3>内存使用率</h3>
                    </div>
                  </template>
                  <div class="chart-content">
                    <div class="mock-chart">
                      <div class="chart-bars">
                        <div v-for="i in 30" :key="i" class="chart-bar mem-bar" :style="{ height: memHistoryData[i-1] + '%' }"></div>
                      </div>
                      <div class="chart-x-axis">
                        <div v-for="i in 6" :key="i" class="x-axis-label">{{ 12 + i }}:00</div>
                      </div>
                    </div>
                  </div>
                </el-card>
              </el-col>

              <el-col :span="12">
                <el-card class="mb-4">
                  <template #header>
                    <div class="chart-header">
                      <h3>网络入流量</h3>
                    </div>
                  </template>
                  <div class="chart-content">
                    <div class="mock-chart">
                      <div class="chart-bars">
                        <div v-for="i in 30" :key="i" class="chart-bar net-in-bar" :style="{ height: netInHistoryData[i-1] + '%' }"></div>
                      </div>
                      <div class="chart-x-axis">
                        <div v-for="i in 6" :key="i" class="x-axis-label">{{ 12 + i }}:00</div>
                      </div>
                    </div>
                  </div>
                </el-card>
              </el-col>

              <el-col :span="12">
                <el-card class="mb-4">
                  <template #header>
                    <div class="chart-header">
                      <h3>网络出流量</h3>
                    </div>
                  </template>
                  <div class="chart-content">
                    <div class="mock-chart">
                      <div class="chart-bars">
                        <div v-for="i in 30" :key="i" class="chart-bar net-out-bar" :style="{ height: netOutHistoryData[i-1] + '%' }"></div>
                      </div>
                      <div class="chart-x-axis">
                        <div v-for="i in 6" :key="i" class="x-axis-label">{{ 12 + i }}:00</div>
                      </div>
                    </div>
                  </div>
                </el-card>
              </el-col>

              <el-col :span="12">
                <el-card class="mb-4">
                  <template #header>
                    <div class="chart-header">
                      <h3>磁盘使用率</h3>
                    </div>
                  </template>
                  <div class="chart-content">
                    <div class="mock-chart">
                      <div class="chart-bars">
                        <div v-for="i in 30" :key="i" class="chart-bar disk-bar" :style="{ height: diskHistoryData[i-1] + '%' }"></div>
                      </div>
                      <div class="chart-x-axis">
                        <div v-for="i in 6" :key="i" class="x-axis-label">{{ 12 + i }}:00</div>
                      </div>
                    </div>
                  </div>
                </el-card>
              </el-col>

              <el-col :span="12">
                <el-card class="mb-4">
                  <template #header>
                    <div class="chart-header">
                      <h3>系统负载</h3>
                    </div>
                  </template>
                  <div class="chart-content">
                    <div class="mock-chart">
                      <div class="chart-bars">
                        <div v-for="i in 30" :key="i" class="chart-bar load-bar" :style="{ height: loadHistoryData[i-1] + '%' }"></div>
                      </div>
                      <div class="chart-x-axis">
                        <div v-for="i in 6" :key="i" class="x-axis-label">{{ 12 + i }}:00</div>
                      </div>
                    </div>
                  </div>
                </el-card>
              </el-col>
            </el-row>
          </div>
        </div>
      </el-drawer>

      <!-- 分页 -->
      <div class="pagination mt-4">
        <div class="pagination-info">
          共 {{ hostTotal }} 台主机
        </div>
        <el-pagination
          background
          layout="prev, pager, next, jumper"
          :total="hostTotal"
          :page-size="pageSize"
          :current-page="hostCurrentPage"
          @current-change="handleCurrentChange"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { useMockData } from '@/composables/useMockData'

const { generateMockData, getProgressColor } = useMockData()

// Props
defineProps<{
  loading?: boolean
  error?: string
}>()

// 搜索表单
const searchForm = ref({
  keyword: ''
})

// 分页相关变量
const pageSize = ref(10)
const hostCurrentPage = ref(1)
const hostTotal = ref(2)

// 右滑框
const drawerVisible = ref(false)
const selectedHost = ref<any>(null)

// 历史时间范围
const historyTimeRange = ref('1h')

// 主机数据
const hosts = ref<any[]>([])

// 历史数据
const cpuHistoryData = ref<number[]>([])
const memHistoryData = ref<number[]>([])
const netInHistoryData = ref<number[]>([])
const netOutHistoryData = ref<number[]>([])
const diskHistoryData = ref<number[]>([])
const loadHistoryData = ref<number[]>([])

// 加载主机数据
const loadHosts = async () => {
  hosts.value = [
    {
      id: '1',
      name: 'vm-CentOS7.6-127.0.0.36',
      ip: '127.0.0.36',
      os: 'CentOS 7.6',
      cpuUsage: 20.8,
      memUsage: 60.5,
      load: 2.19
    },
    {
      id: '2',
      name: 'vm-CentOS7.6-127.0.0.37',
      ip: '127.0.0.37',
      os: 'CentOS 7.6',
      cpuUsage: 5.4,
      memUsage: 80.2,
      load: 2.3
    }
  ]
  hostTotal.value = 2
}

// 搜索处理函数
const handleSearch = async () => {
  await loadHosts()
}

// 分页处理
const handleCurrentChange = async (page: number) => {
  hostCurrentPage.value = page
  await loadHosts()
}

// 行点击事件
const handleRowClick = (row: any) => {
  selectedHost.value = row
  drawerVisible.value = true
}

// 刷新历史数据
const refreshHistory = async () => {
  cpuHistoryData.value = generateMockData(50, 10)
  memHistoryData.value = generateMockData(30, 50)
  netInHistoryData.value = generateMockData(80, 10)
  netOutHistoryData.value = generateMockData(70, 15)
  diskHistoryData.value = generateMockData(20, 40)
  loadHistoryData.value = generateMockData(40, 30)
}

// 初始化
onMounted(() => {
  cpuHistoryData.value = generateMockData(50, 10, 30)
  memHistoryData.value = generateMockData(30, 50, 30)
  netInHistoryData.value = generateMockData(80, 10, 30)
  netOutHistoryData.value = generateMockData(70, 15, 30)
  diskHistoryData.value = generateMockData(20, 40, 30)
  loadHistoryData.value = generateMockData(40, 30, 30)
  loadHosts()
})
</script>

<style scoped>
.host-page {
  width: 100%;
}

.search-filter {
  margin-bottom: 20px;
}

.host-list {
  background-color: white;
  border-radius: 4px;
  padding: 15px;
}

.progress-container {
  position: relative;
  padding-right: 60px;
}

.progress-text {
  position: absolute;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  font-size: 12px;
  color: #606266;
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

/* 右滑框样式 */
.host-detail {
  padding: 20px;
}

.detail-header {
  margin-bottom: 20px;
}

.time-range-select {
  display: flex;
  align-items: center;
  margin-bottom: 20px;
}

.history-charts {
  margin-top: 20px;
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

.net-out-bar {
  background-color: #f56c6c;
}

.disk-bar {
  background-color: #909399;
}

.load-bar {
  background-color: #c0c4cc;
}

.chart-x-axis {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #909399;
}
</style>
