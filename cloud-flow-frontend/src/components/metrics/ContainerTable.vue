<template>
  <div class="container-page">
    <!-- 搜索和筛选-->
    <div class="search-filter mb-4">
      <el-form :inline="true" :model="containerSearchForm" class="demo-form-inline">
        <el-form-item label="时间范围">
          <el-input v-model="containerSearchForm.keyword" placeholder="输入容器名称、命名空间或IP" style="width: 300px;" />
        </el-form-item>
        <el-form-item label="命名空间">
          <el-select v-model="containerSearchForm.namespace" placeholder="全部" style="width: 150px;">
            <el-option label="全部" value="all" />
            <el-option label="default" value="default" />
            <el-option label="kube-system" value="kube-system" />
            <el-option label="app" value="app" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleContainerSearch">搜索</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 容器列表 -->
    <div class="container-list">
      <el-table
        :data="containers"
        style="width: 100%"
        @row-click="handleContainerRowClick"
      >
        <el-table-column prop="podName" label="POD策略" width="200" />
        <el-table-column prop="ip" label="实例IP" width="150" />
        <el-table-column prop="node" label="所属Node" width="180" />
        <el-table-column prop="namespace" label="所在命名空间" width="150" />
        <el-table-column prop="restartCount" label="重启次数" width="100" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="scope.row.status === '运行中' ? 'success' : 'danger'">
              {{ scope.row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="runTime" label="总运行时长" width="150" />
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="scope">
            <el-button size="small" type="primary" @click="handleContainerRowClick(scope.row)">
              查看详情
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 右滑框查看历史指标 -->
      <el-drawer
        v-model="containerDrawerVisible"
        direction="rtl"
        :title="selectedContainer?.podName"
        size="70%"
      >
        <div class="container-detail">
          <div class="detail-header">
            <el-descriptions :column="3">
              <el-descriptions-item label="POD策略">{{ selectedContainer?.podName }}</el-descriptions-item>
              <el-descriptions-item label="实例IP">{{ selectedContainer?.ip }}</el-descriptions-item>
              <el-descriptions-item label="所属Node">{{ selectedContainer?.node }}</el-descriptions-item>
              <el-descriptions-item label="所在命名空间">{{ selectedContainer?.namespace }}</el-descriptions-item>
              <el-descriptions-item label="重启次数">{{ selectedContainer?.restartCount }}</el-descriptions-item>
              <el-descriptions-item label="状态">{{ selectedContainer?.status }}</el-descriptions-item>
              <el-descriptions-item label="总运行时长">{{ selectedContainer?.runTime }}</el-descriptions-item>
            </el-descriptions>

            <!-- Container 切换下拉菜单 -->
            <div class="container-select mt-4">
              <el-form :inline="true" :model="containerForm" class="demo-form-inline">
                <el-form-item label="切换 container:">
                  <el-select v-model="containerForm.selectedContainer" placeholder="选择容器" style="width: 300px;">
                    <el-option
                      v-for="container in selectedContainer?.containers"
                      :key="container.id"
                      :label="container.name"
                      :value="container.id"
                    />
                  </el-select>
                </el-form-item>
              </el-form>
            </div>
          </div>

          <!-- 时间范围选择 -->
          <div class="time-range-select mb-4">
            <el-select v-model="containerHistoryTimeRange" placeholder="最近小时" style="margin-right: 10px;">
              <el-option label="最近1小时" value="1h" />
              <el-option label="最近6小时" value="6h" />
              <el-option label="最近12小时" value="12h" />
              <el-option label="最近24小时" value="24h" />
              <el-option label="最近7天" value="7d" />
            </el-select>
            <el-button type="primary" @click="refreshContainerHistory">
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
                        <div v-for="i in 30" :key="i" class="chart-bar cpu-bar" :style="{ height: containerCpuHistoryData[i-1] + '%' }"></div>
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
                        <div v-for="i in 30" :key="i" class="chart-bar mem-bar" :style="{ height: containerMemHistoryData[i-1] + '%' }"></div>
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
                        <div v-for="i in 30" :key="i" class="chart-bar net-in-bar" :style="{ height: containerNetInHistoryData[i-1] + '%' }"></div>
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
                        <div v-for="i in 30" :key="i" class="chart-bar net-out-bar" :style="{ height: containerNetOutHistoryData[i-1] + '%' }"></div>
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
          共 {{ containerTotal }} 个容器
        </div>
        <el-pagination
          background
          layout="prev, pager, next, jumper"
          :total="containerTotal"
          :page-size="containerPageSize"
          :current-page="containerCurrentPage"
          @current-change="handleContainerCurrentChange"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { useMockData } from '@/composables/useMockData'

const { generateMockData } = useMockData()

// Props
defineProps<{
  loading?: boolean
  error?: string
}>()

// 容器搜索表单
const containerSearchForm = ref({
  keyword: '',
  namespace: 'all'
})

// 容器分页相关变量
const containerPageSize = ref(10)
const containerCurrentPage = ref(1)
const containerTotal = ref(13)

// 容器右滑框
const containerDrawerVisible = ref(false)
const selectedContainer = ref<any>(null)

// 容器表单
const containerForm = ref({
  selectedContainer: ''
})

// 容器历史时间范围
const containerHistoryTimeRange = ref('1h')

// 容器数据
const containers = ref<any[]>([])

// 容器历史图表数据
const containerCpuHistoryData = ref<number[]>([])
const containerMemHistoryData = ref<number[]>([])
const containerNetInHistoryData = ref<number[]>([])
const containerNetOutHistoryData = ref<number[]>([])

// 加载容器数据
const loadContainers = async () => {
  containers.value = [
    {
      id: '1',
      podName: 'alertmanager-668557f55b-7glqg',
      ip: '10.244.2.166',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'kube-system',
      restartCount: 0,
      status: '运行中',
      runTime: '724h23m59s',
      containers: [
        { id: '1-1', name: 'alertmanager' },
        { id: '1-2', name: 'config-reloader' }
      ]
    },
    {
      id: '2',
      podName: 'calico-kube-controllers-7579554b48-2h5cf',
      ip: '10.244.2.165',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'kube-system',
      restartCount: 0,
      status: '运行中',
      runTime: '724h24m14s',
      containers: [
        { id: '2-1', name: 'calico-kube-controllers' }
      ]
    },
    {
      id: '3',
      podName: 'deepflow-server-7566f66859-2v52c',
      ip: '10.244.2.167',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'deepflow',
      restartCount: 0,
      status: '运行中',
      runTime: '472h24m14s',
      containers: [
        { id: '3-1', name: 'deepflow-server' },
        { id: '3-2', name: 'init-container' }
      ]
    },
    {
      id: '4',
      podName: 'deepflow-server-7566f66859-5v78c',
      ip: '10.244.2.168',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'deepflow',
      restartCount: 0,
      status: '运行中',
      runTime: '472h24m14s',
      containers: [
        { id: '4-1', name: 'deepflow-server' },
        { id: '4-2', name: 'init-container' }
      ]
    },
    {
      id: '5',
      podName: 'deepflow-talker-7566f66859-2v52c',
      ip: '10.244.2.169',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'deepflow',
      restartCount: 0,
      status: '运行中',
      runTime: '472h24m14s',
      containers: [
        { id: '5-1', name: 'deepflow-talker' }
      ]
    },
    {
      id: '6',
      podName: 'deepflow-statistics-7566f66859-2v52c',
      ip: '10.244.2.170',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'deepflow',
      restartCount: 0,
      status: '运行中',
      runTime: '472h24m14s',
      containers: [
        { id: '6-1', name: 'deepflow-statistics' }
      ]
    },
    {
      id: '7',
      podName: 'hw-analyzer-w770-7566f66859-2v52c',
      ip: '10.244.2.171',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'app',
      restartCount: 0,
      status: '运行中',
      runTime: '472h24m14s',
      containers: [
        { id: '7-1', name: 'hw-analyzer' }
      ]
    },
    {
      id: '8',
      podName: 'kube-proxy-7566f66859-2v52c',
      ip: '10.244.2.172',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'kube-system',
      restartCount: 0,
      status: '运行中',
      runTime: '724h24m14s',
      containers: [
        { id: '8-1', name: 'kube-proxy' }
      ]
    },
    {
      id: '9',
      podName: 'kube-state-metrics-7566f66859-2v52c',
      ip: '10.244.2.173',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'kube-system',
      restartCount: 0,
      status: '运行中',
      runTime: '724h24m14s',
      containers: [
        { id: '9-1', name: 'kube-state-metrics' }
      ]
    },
    {
      id: '10',
      podName: 'edge-service-ms-#67',
      ip: '10.244.2.174',
      node: 'cn-beijing72.127.0.0.36',
      namespace: 'app',
      restartCount: 0,
      status: '运行中',
      runTime: '472h24m14s',
      containers: [
        { id: '10-1', name: 'edge-service' },
        { id: '10-2', name: 'sidecar' }
      ]
    }
  ]
  containerTotal.value = 13
}

// 容器搜索处理函数
const handleContainerSearch = async () => {
  await loadContainers()
}

// 容器分页处理函数
const handleContainerCurrentChange = async (page: number) => {
  containerCurrentPage.value = page
  await loadContainers()
}

// 容器行点击事件
const handleContainerRowClick = (row: any) => {
  selectedContainer.value = row
  containerForm.value.selectedContainer = row.containers[0].id
  containerDrawerVisible.value = true
}

// 刷新容器历史数据
const refreshContainerHistory = async () => {
  containerCpuHistoryData.value = generateMockData(50, 10)
  containerMemHistoryData.value = generateMockData(30, 50)
  containerNetInHistoryData.value = generateMockData(80, 10)
  containerNetOutHistoryData.value = generateMockData(70, 15)
}

// 初始化
onMounted(() => {
  containerCpuHistoryData.value = generateMockData(50, 10, 30)
  containerMemHistoryData.value = generateMockData(30, 50, 30)
  containerNetInHistoryData.value = generateMockData(80, 10, 30)
  containerNetOutHistoryData.value = generateMockData(70, 15, 30)
  loadContainers()
})
</script>

<style scoped>
.container-page {
  width: 100%;
}

.search-filter {
  margin-bottom: 20px;
}

.container-list {
  background-color: white;
  border-radius: 4px;
  padding: 15px;
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
.container-detail {
  padding: 20px;
}

.detail-header {
  margin-bottom: 20px;
}

.container-select {
  margin-top: 16px;
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

.chart-x-axis {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #909399;
}
</style>
