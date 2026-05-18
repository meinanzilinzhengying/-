<template>

  <div class="log">

 <!-- 日志中心 -->

    <el-card class="mb-4">

      <template #header>

        <div class="card-header">

          <h2>日志中心</h2>

        </div>

      </template>

      

 <!-- 标签页-->

      <el-tabs v-model="activeTab">

 <!-- 日志标签页-->

        <el-tab-pane label="日志" name="log">

 <!-- 时间范围选择 -->

          <div class="time-range-select mb-4">

            <el-select v-model="timeRange" placeholder="最近15分钟" style="margin-right: 10px;">

              <el-option label="最近15分钟" value="5m" />

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

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

            <el-button type="primary" @click="refreshLogs">

              <el-icon><Refresh /></el-icon> 刷新

            </el-button>

            <el-button @click="saveSearchSnapshot">

              <el-icon><Download /></el-icon> 保存快照

            </el-button>

          </div>

          

 <!-- 搜索快照 -->

          <div class="search-snapshot mb-4" v-if="searchSnapshots.length > 0">

            <el-tag

              v-for="snapshot in searchSnapshots"

              :key="snapshot.id"

              closable

              @close="removeSnapshot(snapshot.id)"

              @click="loadSnapshot(snapshot)"

              style="margin-right: 10px; cursor: pointer;"

            >

              {{ snapshot.name }}

            </el-tag>

          </div>

          

 <!-- 搜索保存与设-->

          <div class="search-settings mb-4">

            <el-button @click="saveSearch">保存搜索</el-button>

            <el-dropdown>

              <el-button>

                设置 <el-icon class="el-icon--right"><ArrowDown /></el-icon>

              </el-button>

              <template #dropdown>

                <el-dropdown-menu>

                  <el-dropdown-item @click="viewDatabaseFields">数据库字段</el-dropdown-item>

                  <el-dropdown-item @click="toggleNameDisplay">

                    {{ nameDisplay === 'full' ? '服务名称、请求方法、响应状态、延迟和告警阈值' : '服务名称、请求路径和响应状态码' }}

                  </el-dropdown-item>

                </el-dropdown-menu>

              </template>

            </el-dropdown>

          </div>

          

 <!-- 搜索和筛-->

          <div class="search-filter mb-4">

            <el-form :inline="true" :model="searchForm" class="demo-form-inline">

              <el-form-item label="时间范围">

                <el-select v-model="searchForm.searchType" style="width: 100px; margin-right: 5px;">

                  <el-option label="包含" value="contains" />

                  <el-option label="不包含" value="not_contains" />

                </el-select>

                <el-input v-model="searchForm.keyword" placeholder="输入搜索条件" style="width: 300px;" />

              </el-form-item>

              <el-form-item>

                <el-button type="primary" @click="handleSearch">搜索</el-button>

              </el-form-item>

            </el-form>

          </div>

          

<!-- 服务搜索框和区域查询 -->

          <div class="service-select mb-4">

            <el-form :inline="true" :model="filterForm" class="demo-form-inline">

              <el-form-item label="服务搜索框">

                <el-input v-model="filterForm.service" placeholder="支持对Tag进行搜索或分析" style="width: 300px;" />

              </el-form-item>

              <el-form-item label="区域查询">

                <el-select v-model="filterForm.region" placeholder="全部" @change="handleRegionChange">

                  <el-option label="全部" value="all" />

                  <el-option label="区域1" value="region1" />

                  <el-option label="区域2" value="region2" />

                  <el-option label="区域3" value="region3" />

                </el-select>

              </el-form-item>

            </el-form>

          </div>

          

 <!-- 左侧快速过滤 -->

          <div class="quick-filter mb-4">

            <el-row :gutter="20">

              <el-col :span="8">

                <h3>应用服务</h3>

                <div class="filter-list">

                  <el-checkbox-group v-model="filterForm.services">

                    <el-checkbox label="edge-service-ms-#67">edge-service-ms-#67</el-checkbox>

                    <el-checkbox label="kube-state-metrics">kube-state-metrics</el-checkbox>

                    <el-checkbox label="hw-analyzer-w770">hw-analyzer-w770</el-checkbox>

                    <el-checkbox label="fuser">fuser</el-checkbox>

                    <el-checkbox label="front-end">front-end</el-checkbox>

                    <el-checkbox label="feast">feast</el-checkbox>

                    <el-checkbox label="endpoints-operator">endpoints-operator</el-checkbox>

                    <el-checkbox label="deepflow-talker">deepflow-talker</el-checkbox>

                    <el-checkbox label="deepflow-server">deepflow-server</el-checkbox>

                    <el-checkbox label="deepflow-statistics">deepflow-statistics</el-checkbox>

                  </el-checkbox-group>

                </div>

              </el-col>

              <el-col :span="8">

                <h3>日志级别</h3>

                <div class="filter-list">

                  <el-checkbox-group v-model="filterForm.levels">

                    <el-checkbox label="ERROR" :style="{ color: '#F56C6C' }">ERROR (2%)</el-checkbox>

                    <el-checkbox label="WARN" :style="{ color: '#E6A23C' }">WARN (5%)</el-checkbox>

                    <el-checkbox label="INFO" :style="{ color: '#67C23A' }">INFO (30%)</el-checkbox>

                    <el-checkbox label="UNKNOWN" :style="{ color: '#909399' }">UNKNOWN (63%)</el-checkbox>

                  </el-checkbox-group>

                </div>

              </el-col>

              <el-col :span="8">

                <h3>区域</h3>

                <div class="filter-list">

                  <el-checkbox-group v-model="filterForm.regions">

                    <el-checkbox label="区域1">区域1</el-checkbox>

                    <el-checkbox label="区域2">区域2</el-checkbox>

                    <el-checkbox label="区域3">区域3</el-checkbox>

                  </el-checkbox-group>

                </div>

              </el-col>

            </el-row>

          </div>

          

 <!-- 日志级别过滤 -->

          <div class="log-levels mb-4">

            <el-collapse>

              <el-collapse-item title="日志级别">

                <div class="level-filters">

                  <el-checkbox-group v-model="filterForm.levels">

                    <el-checkbox label="ERROR" :style="{ color: '#F56C6C' }">ERROR (2%)</el-checkbox>

                    <el-checkbox label="WARN" :style="{ color: '#E6A23C' }">WARN (5%)</el-checkbox>

                    <el-checkbox label="INFO" :style="{ color: '#67C23A' }">INFO (30%)</el-checkbox>

                    <el-checkbox label="UNKNOWN" :style="{ color: '#909399' }">UNKNOWN (63%)</el-checkbox>

                  </el-checkbox-group>

                </div>

              </el-collapse-item>

            </el-collapse>

          </div>

          

 <!-- 服务搜索和区域过滤 -->

          <div class="chart-container mb-4">

            <div class="chart-header">

              <h3>异常分析</h3>

            </div>

            <div class="chart-content">

              <div class="mock-chart">

                <div class="chart-bars">

                  <div v-for="(height, index) in logChartData" :key="index" class="chart-bar" :style="{ height: height + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">{{ 11 + i }}:34</div>

                </div>

              </div>

            </div>

          </div>

          

 <!-- 请求日志列表 -->

          <div class="log-table">

            <div class="table-header">

              <div class="table-title">日志详情（共137,759条）</div>

              <div class="table-controls">

                <el-button type="primary" @click="exportLogs">导出</el-button>

                <el-select v-model="autoRefresh" placeholder="自动(2s)" style="margin-left: 10px;">

                  <el-option label="关闭" value="off" />

                  <el-option label="自动(2s)" value="2s" />

                  <el-option label="自动(5s)" value="5s" />

                  <el-option label="自动(10s)" value="10s" />

                </el-select>

              </div>

            </div>

            

            <el-table 

              :data="logs" 

              style="width: 100%"

              @row-click="handleRowClick"

            >

              <el-table-column prop="time" label="时间" width="180" sortable />

              <el-table-column prop="level" label="日志级别" width="100">

                <template #default="scope">

                  <el-tag :type="getLevelType(scope.row.level)" :style="{ color: getLevelColor(scope.row.level) }">

                    {{ scope.row.level }}

                  </el-tag>

                </template>

              </el-table-column>

              <el-table-column prop="description" label="日志说明" width="150" />

              <el-table-column prop="content" label="日志内容" min-width="400" />

              <el-table-column prop="service" label="应用服务" width="150" />

              <el-table-column label="操作" width="100" fixed="right">

                <template #default="scope">

                  <el-dropdown>

                    <el-button size="small">

                      操作 <el-icon class="el-icon--right"><ArrowDown /></el-icon>

                    </el-button>

                    <template #dropdown>

                      <el-dropdown-menu>

                        <el-dropdown-item @click="copyContent(scope.row, 'plain')">复制为纯文本</el-dropdown-item>

                        <el-dropdown-item @click="copyContent(scope.row, 'json')">复制为JSON</el-dropdown-item>

                      </el-dropdown-menu>

                    </template>

                  </el-dropdown>

                </template>

              </el-table-column>

            </el-table>

            

 <!-- 抽屉可以查看应用服务信息-->

            <el-drawer

              v-model="drawerVisible"

              direction="rtl"

              title="调用链追踪和网络流量监控"

              size="50%"

            >

              <div class="service-detail">

                <h3>{{ selectedService?.service }}</h3>

                <el-descriptions :column="1">

                  <el-descriptions-item label="服务名称">{{ selectedService?.service }}</el-descriptions-item>

                  <el-descriptions-item label="日志级别">{{ selectedService?.level }}</el-descriptions-item>

                  <el-descriptions-item label="最近日志参考">{{ selectedService?.time }}</el-descriptions-item>

                  <el-descriptions-item label="日志内容">{{ selectedService?.content }}</el-descriptions-item>

                </el-descriptions>

                <h4>相关日志</h4>

                <el-table :data="relatedLogs" style="width: 100%">

                  <el-table-column prop="time" label="时间" width="180" />

                  <el-table-column prop="level" label="日志级别" width="100">

                    <template #default="scope">

                      <el-tag :type="getLevelType(scope.row.level)" :style="{ color: getLevelColor(scope.row.level) }">

                        {{ scope.row.level }}

                      </el-tag>

                    </template>

                  </el-table-column>

                  <el-table-column prop="content" label="日志内容" min-width="400" />

                </el-table>

              </div>

            </el-drawer>

            

 <!-- 数据库表 -->

            <div class="pagination mt-4">

              <div class="pagination-info">

                共 {{ logTotal }} 条

              </div>

              <el-pagination

                background

                layout="prev, pager, next, jumper"

                :total="logTotal"

                :page-size="pageSize"

                :current-page="currentPage"

                @current-change="handleCurrentChange"

              />

            </div>

          </div>

        </el-tab-pane>

        

 <!-- 日志分析标签页-->

        <el-tab-pane label="日志分析" name="analysis">

          <div class="analysis-content">

            <p>日志分析功能开发中...</p>

          </div>

        </el-tab-pane>

      </el-tabs>

    </el-card>

  </div>

</template>



<script setup lang="ts">

// 生成模拟数据库（仅在组件挂载时调用一次，避免图表跳动）
const generateMockData = (max: number, min: number, count: number = 30) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

import { ref, computed, onMounted, watch, onUnmounted } from 'vue'

import { useRoute } from 'vue-router'

import { ElMessageBox, ElMessage } from 'element-plus'

import { Refresh, Download, ArrowDown } from '@element-plus/icons-vue'



// 路由

const route = useRoute()



// 当前标签页

const activeTab = ref('log')



// 搜索快照管理

const timeRange = ref('5m')

const dateRange = ref<string[]>([])



// 搜索表单

const searchForm = ref({

  searchType: 'contains',

  keyword: ''

})



// 数据源 - 存储CPU、内存、网络等历史数据
const logChartData = ref([])

onMounted(() => {
  logChartData.value = generateMockData(80, 20, 30)
})


// 过滤表单
const filterForm = ref({

  service: '',

  region: 'all',

  levels: ['ERROR', 'WARN', 'INFO', 'UNKNOWN'],

  services: [],

  regions: []

})



// 自动刷新

const autoRefresh = ref('2s')

// 自动刷新定时器
let refreshTimer: ReturnType<typeof setInterval> | null = null
watch(autoRefresh, (val) => {
  if (refreshTimer) clearInterval(refreshTimer)
  if (val && val !== 'off') {
    const ms = parseInt(val) * 1000
    refreshTimer = setInterval(() => { handleSearch() }, ms)
  }
})
onUnmounted(() => { if (refreshTimer) clearInterval(refreshTimer) })



// 分页相关变量

const pageSize = ref(10)

const currentPage = ref(1)



// 搜索快照

const searchSnapshots = ref([

  { id: '1', name: '快照1' },

  { id: '2', name: '快照2' }

])



// 名称显示模式切换

const nameDisplay = ref('full')



// 右滑
const drawerVisible = ref(false)

const selectedService = ref<any>(null)

const relatedLogs = ref<any[]>([])



// 服务搜索和区域过滤

const logs = ref([

  {

    id: '1',

    time: '2024-06-19 11:47:07',

    level: 'UNKNOWN',

    description: 'UNKNOWN',

    content: '1606-19 11:47:07.066882 1 round.trippers.go:454] GET http://192.168.0.1:443/api/v1/namespaces',

    service: 'endpoints-operator'

  },

  {

    id: '2',

    time: '2024-06-19 11:47:07',

    level: 'INFO',

    description: 'INFO',

    content: '2024-06-19 11:47:07.944 [INFO] [trisolaris/synchronize] remote.execute.go:121 agent(key: deepflow-server',

    service: 'deepflow-server'

  },

  {

    id: '3',

    time: '2024-06-19 11:47:07',

    level: 'INFO',

    description: 'INFO',

    content: '2024-06-19 11:47:07.944 [INFO] [trisolaris/synchronize] remote.execute.go:139 (key: deepflow-server',

    service: 'deepflow-server'

  },

  {

    id: '4',

    time: '2024-06-19 11:47:07',

    level: 'INFO',

    description: 'INFO',

    content: '2024-06-19 11:47:07.679 [INFO] [trisolaris/metadata] metadata.go:199 ORG-ID: 1; start generat',

    service: 'deepflow-server'

  },

  {

    id: '5',

    time: '2024-06-19 11:47:07',

    level: 'UNKNOWN',

    description: 'UNKNOWN',

    content: '2024-06-19 11:47:07.679 [INFO] [124.74.174.145 PUT] /caches/organization-18c8f2ea-platform-data',

    service: 'deepflow-server'

  },

  {

    id: '6',

    time: '2024-06-19 11:47:07',

    level: 'UNKNOWN',

    description: 'UNKNOWN',

    content: '2024-06-19 11:47:07.667 [INFO] [124.74.174.145 PUT] /caches/organization-18c8f2ea-platform-data',

    service: 'deepflow-server'

  },

  {

    id: '7',

    time: '2024-06-19 11:47:07',

    level: 'INFO',

    description: 'INFO',

    content: '2024-06-19 11:47:07.295 [INFO] [cloud.huawei.token] token.go:155 exclude project (name=ai-fout',

    service: 'deepflow-server'

  },

  {

    id: '8',

    time: '2024-06-19 11:47:07',

    level: 'INFO',

    description: 'INFO',

    content: '2024-06-19 11:47:07.225 [INFO] [trisolaris/synchronize] remote.execute.go:139 (key: deepflow-server',

    service: 'deepflow-server'

  },

  {

    id: '9',

    time: '2024-06-19 11:47:07',

    level: 'INFO',

    description: 'INFO',

    content: '2024-06-19 11:47:07.087 [INFO] [trisolaris/vtap] vtap.go:1285 ORG-ID: 1; end generate gpid',

    service: 'deepflow-server'

  },

  {

    id: '10',

    time: '2024-06-19 11:47:07',

    level: 'INFO',

    description: 'INFO',

    content: '2024-06-19 11:47:07.083 [INFO] [trisolaris/vtap] vtap.go:1289 ORG-ID: 1; start generate gpid',

    service: 'deepflow-server'

  }

])



const logTotal = ref(137759)



// 分页处理函数

const handleSearch = () => {
  ElMessage.info('功能开发中...')
}



// 刷新操作

const refreshLogs = () => {
  ElMessage.info('功能开发中...')
}



// 导出日志

const exportLogs = () => {
  ElMessage.info('功能开发中...')
}



// 分页流程处理

const handleCurrentChange = (page: number) => {

  currentPage.value = page

  }



// 获取级别类型

const getLevelType = (level: string) => {

  switch (level) {

    case 'ERROR':

      return 'danger'

    case 'WARN':

      return 'warning'

    case 'INFO':

      return 'success'

    default:

      return 'info'

  }

}



// 获取级别颜色

const getLevelColor = (level: string) => {

  switch (level) {

    case 'ERROR':

      return '#F56C6C'

    case 'WARN':

      return '#E6A23C'

    case 'INFO':

      return '#67C23A'

    default:

      return '#909399'

  }

}



// 搜索快照列表

const saveSearchSnapshot = async () => {

  try {

    const snapshotName = await ElMessageBox.prompt('请输入快照名称', '保存快照', {

      confirmButtonText: '保存',

      cancelButtonText: '取消',

      inputPattern: /\S+/,

      inputErrorMessage: '快照名称不能为空'

    })

    if (snapshotName.value) {

      searchSnapshots.value.push({

        id: Date.now().toString(),

        name: snapshotName.value

      })

    }

  } catch {

// 表格行点击事件处理

  }

}



const loadSnapshot = (snapshot: any) => {
  ElMessage.info('功能开发中...')
}



const removeSnapshot = (id: string) => {

  searchSnapshots.value = searchSnapshots.value.filter(s => s.id !== id)

  }



// 搜索保存与设置操作
const saveSearch = () => {
  ElMessage.info('功能开发中...')
}



const viewDatabaseFields = () => {

 // 这里可以显示数据库字段的弹窗

}



const toggleNameDisplay = () => {

  nameDisplay.value = nameDisplay.value === 'full' ? 'short' : 'full'

  }



// 区域查询变更

const handleRegionChange = (region: string) => {
  ElMessage.info('功能开发中...')
}



// 表格操作按钮

const handleRowClick = (row: any) => {

  selectedService.value = row

// 模块流量监控和告警详情展示

  relatedLogs.value = logs.value.filter(log => log.service === row.service).slice(0, 5)

  drawerVisible.value = true

  }



const copyContent = (row: any, format: string) => {

  let content = ''

  if (format === 'plain') {

    content = `时间范围？: ${row.time}\n级别: ${row.level}\n内容: ${row.content}\n服务: ${row.service}`

  } else if (format === 'json') {

    content = JSON.stringify(row, null, 2)

  }

  navigator.clipboard.writeText(content).then(() => {
      ElMessage.success('复制成功')
    }).catch(() => {
      ElMessage.error('复制失败，请手动复制')
    })
}



</script>



<style scoped>

.log {

  padding: 20px;

}



.card-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.time-range-select {

  display: flex;

  align-items: center;

  margin-bottom: 20px;

}



.search-filter {

  margin-bottom: 20px;

}



.service-select {

  margin-bottom: 20px;

}



.log-levels {

  margin-bottom: 20px;

}



.level-filters {

  display: flex;

  gap: 20px;

}



.chart-container {

  background-color: #f5f7fa;

  padding: 15px;

  border-radius: 4px;

  margin-bottom: 20px;

}



.chart-header h3 {

  margin: 0 0 15px 0;

  font-size: 16px;

  font-weight: bold;

}



.mock-chart {

  background-color: white;

  padding: 20px;

  border-radius: 4px;

  height: 150px;

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

  background-color: #409eff;

  border-radius: 4px 4px 0 0;

  min-height: 10px;

}



.chart-x-axis {

  display: flex;

  justify-content: space-between;

  font-size: 12px;

  color: #909399;

}



.log-table {

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

  align-items: center;

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



.analysis-content {

  padding: 20px;

  text-align: center;

  color: #909399;

}



/* 搜索快照样式 */

.search-snapshot {

  margin-bottom: 20px;

  padding: 10px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



/* 最近5分钟设置样式 */

.search-settings {

  margin-bottom: 20px;

  display: flex;

  gap: 10px;

}



/* 快速过滤样?*/

.quick-filter {

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.quick-filter h3 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 14px;

  font-weight: bold;

}



.filter-list {

  max-height: 200px;

  overflow-y: auto;

}



.filter-list .el-checkbox {

  display: block;

  margin-bottom: 5px;

}



/* 服务详情样式 */

.service-detail {

  padding: 20px;

}



.service-detail h3 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 18px;

  font-weight: bold;

}



.service-detail h4 {

  margin-top: 20px;

  margin-bottom: 10px;

  font-size: 16px;

  font-weight: bold;

}



</style>