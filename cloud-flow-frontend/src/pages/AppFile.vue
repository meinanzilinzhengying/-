<template>

  <div class="app-file-content">

 <!-- TCP重传详情-->

    <div class="file-header">

      <div class="file-search">

        <el-form :inline="true" :model="fileForm" class="demo-form-inline">

          <el-form-item label="时间范围">

            <el-select v-model="fileForm.snapshot" placeholder="查询快照" style="width: 200px;">

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="fileForm.search" placeholder="搜索关键词" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchFile">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="file-actions">

        <el-form :inline="true" :model="fileActionsForm" class="demo-form-inline">

          <el-form-item>

            <el-button @click="saveSearch">

              保存搜索条件

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="refreshFile">

              刷新

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="exportFile">

              导出数据库

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 业务监控-->

    <div class="file-content">

 <!-- 左侧快速过滤-->

      <div class="file-sidebar">

 <!-- 事件类型 -->

        <div class="filter-section">

          <h3>事件类型</h3>

          <el-checkbox-group v-model="selectedEventTypes">

            <el-checkbox label="读">读</el-checkbox>

            <el-checkbox label="写">写</el-checkbox>

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

      <div class="file-main">

 <!-- 图表数据库展示 -->

        <div class="file-chart">

          <el-card>

            <template #header>

              <div class="chart-header">

                <h3>异常分析</h3>

              </div>

            </template>

            <div class="mock-chart file-chart-content">

              <div class="chart-bars">

                <div v-for="i in 60" :key="i" class="chart-bar file-bar" :style="{ height: trendData[i-1] + '%' }"></div>

              </div>

              <div class="chart-x-axis">

                <div v-for="i in 6" :key="i" class="x-axis-label">13:24</div>

              </div>

            </div>

          </el-card>

        </div>

        

 <!-- 文件读写列表〃 -->

        <div class="file-list">

          <div class="table-header">

            <h3>事件详情列表</h3>

            <div class="table-actions">

              <el-dropdown>

                <el-button size="small">

                  列选择

                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>

                </el-button>

                <template #dropdown>

                  <el-dropdown-menu>

                    <el-dropdown-item @click="toggleColumn('startTime')">开始时间</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('eventType')">事件类型</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('operation')">吞吐量</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('process')">进程/容器/应用名称</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('applicationId')">应用名称</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('eventInfo')">事件信息</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('filePath')">文件路径</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('endTime')">结束时间</el-dropdown-item>

                  </el-dropdown-menu>

                </template>

              </el-dropdown>

            </div>

          </div>

          <el-table :data="fileData" style="width: 100%" @row-click="handleFileRowClick">

            <el-table-column prop="startTime" label="开始时间" width="180" />

            <el-table-column prop="eventType" label="事件类型" width="80" />

            <el-table-column prop="operation" label="操作" width="80" />

            <el-table-column prop="process" label="进程/容器/应用名称" width="180" />

            <el-table-column prop="applicationId" label="应用名称" width="120" />

            <el-table-column prop="eventInfo" label="事件信息" />

            <el-table-column prop="filePath" label="文件路径" width="200" />

            <el-table-column prop="endTime" label="结束时间" width="180" />

          </el-table>

          

 <!-- 数据库表 -->

          <div class="pagination mt-4">

            <div class="pagination-info">

              共 {{ fileTotal }} 条

            </div>

            <el-pagination

              background

              layout="prev, pager, next, jumper"

              :total="fileTotal"

              :page-size="filePageSize"

              :current-page="fileCurrentPage"

              @current-change="handleFilePageChange"

            />

          </div>

        </div>

      </div>

    </div>

    

 <!-- 抽屉-->

    <el-drawer

      v-model="fileDrawerVisible"

      title="文件读写网络流量监控"

      direction="rtl"

      size="50%"

    >

      <div class="file-drawer">

        <h3>文件读写网络流量监控</h3>

        <el-descriptions :column="1" border>

          <el-descriptions-item label="开始时间">{{ selectedFile.startTime }}</el-descriptions-item>

          <el-descriptions-item label="事件类型">{{ selectedFile.eventType }}</el-descriptions-item>

          <el-descriptions-item label="操作">{{ selectedFile.operation }}</el-descriptions-item>

          <el-descriptions-item label="进程/容器/应用名称">{{ selectedFile.process }}</el-descriptions-item>

          <el-descriptions-item label="应用名称">{{ selectedFile.applicationId }}</el-descriptions-item>

          <el-descriptions-item label="事件信息">{{ selectedFile.eventInfo }}</el-descriptions-item>

          <el-descriptions-item label="文件路径">{{ selectedFile.filePath }}</el-descriptions-item>

          <el-descriptions-item label="结束时间">{{ selectedFile.endTime }}</el-descriptions-item>

        </el-descriptions>

        <div class="mt-4">

          <h4>关键指标</h4>

          <div class="drawer-charts">

            <div class="drawer-chart">

              <h5>读写吞吐量</h5>

              <div class="mock-chart drawer-chart-content">

                <div class="chart-bars">

                  <div v-for="i in 20" :key="i" class="chart-bar drawer-bar" :style="{ height: readWriteTimeData[i-1] + '%' }"></div>

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



// 文件读写监控表单

const fileForm = ref({

  snapshot: '15m',

  search: ''

})



const fileActionsForm = ref({})



// 事件类型选择

const selectedEventTypes = ref(['读', '写'])



// 区域查询

const regionQuery = ref('全部')



// 图表数据库流

const trendData = ref([])

const readWriteTimeData = ref([])

onMounted(() => {
  trendData.value = generateMockData(80, 20, 60)
  readWriteTimeData.value = generateMockData(90, 10, 20)
})




// 文件读写列表〃数据流

const fileData = ref([

  {

    startTime: '2023-09-21 15:08:36.947975',

    eventType: 'IO',

    operation: '读',

    process: 'cn-changchun',

    applicationId: '1707031',

    eventInfo: 'process runc (1707031) read 33989 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:36.948082'

  },

  {

    startTime: '2023-09-21 15:08:35.523535',

    eventType: 'IO',

    operation: '读',

    process: 'java-eu',

    applicationId: '3159951',

    eventInfo: 'process pool-1-thread-1 (3159951) read 327 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.524696'

  },

  {

    startTime: '2023-09-21 15:08:35.170463',

    eventType: 'IO',

    operation: '写',

    process: 'cn-changchun',

    applicationId: '1707031',

    eventInfo: 'process runc (1707031) wrote 6 bytes and took 1ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.171091'

  },

  {

    startTime: '2023-09-21 15:08:35.026568',

    eventType: 'IO',

    operation: '读',

    process: '0.0.0.0',

    applicationId: '3708146',

    eventInfo: 'process socket-syncron (3708146) read 2246 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.026688'

  },

  {

    startTime: '2023-09-21 15:08:35.026471',

    eventType: 'IO',

    operation: '读',

    process: '0.0.0.0',

    applicationId: '3708146',

    eventInfo: 'process socket-syncron (3708146) read 150 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.026544'

  },

  {

    startTime: '2023-09-21 15:08:35.026361',

    eventType: 'IO',

    operation: '读',

    process: '0.0.0.0',

    applicationId: '3708146',

    eventInfo: 'process socket-syncron (3708146) read 4690 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.026476'

  },

  {

    startTime: '2023-09-21 15:08:35.026218',

    eventType: 'IO',

    operation: '读',

    process: '0.0.0.0',

    applicationId: '3708146',

    eventInfo: 'process socket-syncron (3708146) read 2085 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.026363'

  },

  {

    startTime: '2023-09-21 15:08:35.026096',

    eventType: 'IO',

    operation: '读',

    process: '0.0.0.0',

    applicationId: '3708146',

    eventInfo: 'process socket-syncron (3708146) read 3562 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.026224'

  },

  {

    startTime: '2023-09-21 15:08:35.025966',

    eventType: 'IO',

    operation: '读',

    process: '0.0.0.0',

    applicationId: '3708146',

    eventInfo: 'process socket-syncron (3708146) read 3347 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.026098'

  },

  {

    startTime: '2023-09-21 15:08:35.025845',

    eventType: 'IO',

    operation: '读',

    process: '0.0.0.0',

    applicationId: '3708146',

    eventInfo: 'process socket-syncron (3708146) read 3300 bytes and took 0ms',

    filePath: 'cn-changchun',

    endTime: '2023-09-21 15:08:35.025968'

  }

])



// 数据库表详情

const filePageSize = ref(10)

const fileCurrentPage = ref(1)

const fileTotal = ref(50)



// 右侧抽屉弹窗

const fileDrawerVisible = ref(false)

const selectedFile = ref({

  startTime: '',

  eventType: '',

  operation: '',

  process: '',

  applicationId: '',

  eventInfo: '',

  filePath: '',

  endTime: ''

})



// 搜索文件调用链

const searchFile = () => {
  ElMessage.info('功能开发中...')
}



// 保存搜索条件

const saveSearch = () => {
  ElMessage.info('功能开发中...')
}



// 刷新文件调用链

const refreshFile = () => {
  ElMessage.info('功能开发中...')
}



// 导出文件调用链

const exportFile = () => {
  ElMessage.info('功能开发中...')
}



// 切换数据源功能
const toggleColumn = (column: string) => {
  ElMessage.info('功能开发中...')
}



// 处理文件读写行点击
const handleFileRowClick = (row: any) => {

  selectedFile.value = row

  fileDrawerVisible.value = true

  }



// 文件读写事件表变化

const handleFilePageChange = (page: number) => {

  fileCurrentPage.value = page

  }

</script>



<style scoped>

.app-file-content {

  padding: 20px;

}



.file-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.file-search {

  flex: 1;

}



.file-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.file-content {

  display: flex;

  gap: 20px;

}



.file-sidebar {

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



.file-main {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.file-chart {

  margin-bottom: 30px;

}



.chart-header h3 {

  margin: 0;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.file-chart-content {

  height: 200px;

}



.file-bar {

  background-color: #409eff;

  border-radius: 2px 2px 0 0;

}



.file-list {

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



.file-list .el-table {

  margin-bottom: 20px;

}



.file-list .el-table th {

  background-color: #f5f7fa;

}



.file-list .el-table td {

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



.file-drawer {

  padding: 20px;

}



.file-drawer h3 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.file-drawer h4 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.file-drawer h5 {

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

  .file-content {

    flex-direction: column;

  }

  

  .file-sidebar {

    width: 100%;

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