<template>

  <div class="app-log-content">

 <!-- TCP重传详情-->

    <div class="log-header">

      <div class="log-search">

        <el-form :inline="true" :model="logForm" class="demo-form-inline">

          <el-form-item label="时间范围">

            <el-select v-model="logForm.snapshot" placeholder="查询快照" style="width: 200px;">

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="logForm.search" placeholder="搜索关键词" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchLog">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="log-actions">

        <el-form :inline="true" :model="logActionsForm" class="demo-form-inline">

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

            <el-button @click="refreshLog">

              刷新

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="exportLog">

              导出数据库

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 业务监控-->

    <div class="log-content">

 <!-- 左侧快速过滤-->

      <div class="log-sidebar">

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

            <el-checkbox label="DNS">DNS</el-checkbox>

            <el-checkbox label="HTTP">HTTP</el-checkbox>

            <el-checkbox label="TLS">TLS</el-checkbox>

            <el-checkbox label="Redis">Redis</el-checkbox>

            <el-checkbox label="gRPC">gRPC</el-checkbox>

            <el-checkbox label="HTTP2">HTTP2</el-checkbox>

            <el-checkbox label="NTP">NTP</el-checkbox>

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

      <div class="log-main">

 <!-- 图表数据库展示 -->

        <div class="log-chart">

          <el-card>

            <template #header>

              <div class="chart-header">

                <h3>异常分析</h3>

              </div>

            </template>

            <div class="mock-chart log-chart-content">

              <div class="chart-bars">

                <div v-for="i in 60" :key="i" class="chart-bar log-bar" :style="{ height: trendData[i-1] + '%' }"></div>

              </div>

              <div class="chart-x-axis">

                <div v-for="i in 6" :key="i" class="x-axis-label">10:03</div>

              </div>

            </div>

          </el-card>

        </div>

        

 <!-- 调用日志列表 -->

        <div class="log-list">

          <div class="table-header">

            <h3>调用日志列表</h3>

            <div class="table-actions">

              <el-dropdown>

                <el-button size="small">

                  列选择

                  <el-icon class="el-icon--right"><ArrowDown /></el-icon>

                </el-button>

                <template #dropdown>

                  <el-dropdown-menu>

                    <el-dropdown-item @click="toggleColumn('startTime')">开始时间</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('endTime')">结束时间</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('client')">客户端服务</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('server')">服务端</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('application')">应用名称</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('requestType')">请求类型</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('method')">请求方法</el-dropdown-item>

                    <el-dropdown-item @click="toggleColumn('requestContent')">请求内容</el-dropdown-item>

                  </el-dropdown-menu>

                </template>

              </el-dropdown>

            </div>

          </div>

          <el-table :data="logData" style="width: 100%" @row-click="handleLogRowClick">

            <el-table-column prop="startTime" label="开始时间" width="180" />

            <el-table-column prop="endTime" label="结束时间" width="180" />

            <el-table-column prop="client" label="客户端服务" width="150" />

            <el-table-column prop="server" label="服务端列表" width="150" />

            <el-table-column prop="application" label="应用名称" width="120" />

            <el-table-column prop="requestType" label="请求类型" width="120" />

            <el-table-column prop="method" label="请求方法" width="100" />

            <el-table-column prop="requestContent" label="请求内容" />

            <el-table-column label="操作" width="100" fixed="right">

              <template #default="scope">

                <el-dropdown>

                  <el-button size="small">

                    吞吐量

                    <el-icon class="el-icon--right"><ArrowDown /></el-icon>

                  </el-button>

                  <template #dropdown>

                    <el-dropdown-menu>

                      <el-dropdown-item @click="copyLog(scope.row)">复制</el-dropdown-item>

                    </el-dropdown-menu>

                  </template>

                </el-dropdown>

              </template>

            </el-table-column>

          </el-table>

          

 <!-- 数据库表 -->

          <div class="pagination mt-4">

            <div class="pagination-info">

              共 {{ logTotal }} 条

            </div>

            <el-pagination

              background

              layout="prev, pager, next, jumper"

              :total="logTotal"

              :page-size="logPageSize"

              :current-page="logCurrentPage"

              @current-change="handleLogPageChange"

            />

          </div>

        </div>

      </div>

    </div>

    

 <!-- 抽屉-->

    <el-drawer

      v-model="logDrawerVisible"

      title="调用日志详情"

      direction="rtl"

      size="50%"

    >

      <div class="log-drawer">

        <h3>调用日志详情</h3>

        <el-descriptions :column="1" border>

          <el-descriptions-item label="开始时间">{{ selectedLog.startTime }}</el-descriptions-item>

          <el-descriptions-item label="结束时间">{{ selectedLog.endTime }}</el-descriptions-item>

          <el-descriptions-item label="客户端服务">{{ selectedLog.client }}</el-descriptions-item>

          <el-descriptions-item label="服务端列表">{{ selectedLog.server }}</el-descriptions-item>

          <el-descriptions-item label="应用名称">{{ selectedLog.application }}</el-descriptions-item>

          <el-descriptions-item label="请求类型">{{ selectedLog.requestType }}</el-descriptions-item>

          <el-descriptions-item label="请求方法">{{ selectedLog.method }}</el-descriptions-item>

          <el-descriptions-item label="请求内容">{{ selectedLog.requestContent }}</el-descriptions-item>

          <el-descriptions-item label="响应状态">{{ selectedLog.status }}</el-descriptions-item>

          <el-descriptions-item label="分组聚合详情">{{ selectedLog.responseTime }}</el-descriptions-item>

        </el-descriptions>

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

    

 <!-- 复制对话框-->

    <el-dialog

      v-model="copyDialogVisible"

      title="复制内容"

      width="500px"

    >

      <div class="copy-dialog">

        <el-radio-group v-model="copyFormat">

          <el-radio label="text">纯文本格式</el-radio>

          <el-radio label="json">JSON格式</el-radio>

          <el-radio label="csv">CSV格式</el-radio>

        </el-radio-group>

        <el-input

          v-model="copyContent"

          type="textarea"

          :rows="10"

          style="margin-top: 15px;"

          readonly

        />

      </div>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="copyDialogVisible = false">取消</el-button>

          <el-button type="primary" @click="copyToClipboard">复制到剪贴板</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

// 生成模拟数据库（仅在组件挂载时调用一次，避免图表跳动）
const generateMockData = (max: number, min: number, count: number = 30) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

import { ref } from 'vue'

import { ArrowDown } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'



// 调用链监控延迟监控

const logForm = ref({

  snapshot: '15m',

  search: ''

})



const logActionsForm = ref({})



// 信号源选择

const selectedSignalSources = ref(['Packet', 'eBPF'])



// 响应状态选择

const selectedStatuses = ref(['正常', '服务端错误率'])



// 应用协议选择

const selectedProtocols = ref(['MySQL', 'HTTP'])



// 区域查询

const regionQuery = ref('全部')



// 图表数据库流

const trendData = ref([])

const responseTimeData = ref([])

onMounted(() => {
  trendData.value = generateMockData(80, 20, 60)
  responseTimeData.value = generateMockData(90, 10, 20)
})




// 调用日志列表数据流

const logData = ref([

  {

    startTime: '04-03 18:07:58.998',

    endTime: '04-03 18:07:58.998',

    client: '北京办公室VyoS-10.32',

    server: 'cn-beijing,19.1.21.69',

    application: 'MySQL',

    requestType: 'CON_STMT_PREPARE',

    method: '--',

    requestContent: 'SELECT * FROM `vtap` WHERE',

    status: '正常',

    responseTime: '0.5 ms'

  },

  {

    startTime: '04-03 18:07:58.998',

    endTime: '04-03 18:07:58.998',

    client: '北京办公室VyoS-10.32',

    server: 'mysql-deployment-694cd6f6f9-n9nr',

    application: 'MySQL',

    requestType: 'CON_STMT_PREPARE',

    method: '--',

    requestContent: 'SELECT * FROM `vtap` WHERE',

    status: '正常',

    responseTime: '0.3 ms'

  },

  {

    startTime: '04-03 18:07:58.998',

    endTime: '04-03 18:07:58.998',

    client: '北京办公室VyoS-10.32',

    server: 'mysql-deployment-694cd6f6f9-n9nr',

    application: 'MySQL',

    requestType: 'CON_STMT_CLOSE',

    method: '--',

    requestContent: '--',

    status: '正常',

    responseTime: '0.1 ms'

  },

  {

    startTime: '04-03 18:07:58.998',

    endTime: '04-03 18:07:58.998',

    client: '北京办公室VyoS-10.32',

    server: 'cn-beijing,19.1.21.69',

    application: 'MySQL',

    requestType: 'CON_STMT_CLOSE',

    method: '--',

    requestContent: 'Application Data',

    status: '正常',

    responseTime: '0.2 ms'

  },

  {

    startTime: '04-03 18:07:58.996',

    endTime: '04-03 18:07:58.996',

    client: '19.1.0.28',

    server: 'gatekeeper-958c8744c-2lwvk',

    application: 'TLS',

    requestType: 'Application Data',

    method: '--',

    requestContent: 'Application Data',

    status: '正常',

    responseTime: '0.4 ms'

  },

  {

    startTime: '04-03 18:07:58.991',

    endTime: '04-03 18:07:58.991',

    client: '北京办公室VyoS-10.32',

    server: 'mysql-deployment-694cd6f6f9-n9nr',

    application: 'MySQL',

    requestType: 'CON_STMT_EXECUTE',

    method: '--',

    requestContent: '--',

    status: '正常',

    responseTime: '0.3 ms'

  },

  {

    startTime: '04-03 18:07:58.991',

    endTime: '04-03 18:07:58.991',

    client: '北京办公室VyoS-10.32',

    server: 'cn-beijing,19.1.21.69',

    application: 'MySQL',

    requestType: 'CON_STMT_EXECUTE',

    method: '--',

    requestContent: '--',

    status: '正常',

    responseTime: '0.2 ms'

  },

  {

    startTime: '04-03 18:07:58.983',

    endTime: '04-03 18:07:58.983',

    client: '北京办公室VyoS-10.32',

    server: 'cn-beijing,19.1.21.69',

    application: 'MySQL',

    requestType: 'CON_STMT_PREPARE',

    method: '--',

    requestContent: 'SELECT * FROM `vtap` WHERE',

    status: '正常',

    responseTime: '0.5 ms'

  },

  {

    startTime: '04-03 18:07:58.983',

    endTime: '04-03 18:07:58.983',

    client: '北京办公室VyoS-10.32',

    server: 'mysql-deployment-694cd6f6f9-n9nr',

    application: 'MySQL',

    requestType: 'CON_STMT_PREPARE',

    method: '--',

    requestContent: 'SELECT * FROM `vtap` WHERE',

    status: '正常',

    responseTime: '0.3 ms'

  },

  {

    startTime: '04-03 18:07:58.983',

    endTime: '04-03 18:07:58.983',

    client: '北京办公室VyoS-10.32',

    server: 'mysql-deployment-694cd6f6f9-n9nr',

    application: 'MySQL',

    requestType: 'CON_STMT_CLOSE',

    method: '--',

    requestContent: '--',

    status: '正常',

    responseTime: '0.1 ms'

  }

])



// 数据库表详情

const logPageSize = ref(10)

const logCurrentPage = ref(1)

const logTotal = ref(50)



// 右侧抽屉弹窗

const logDrawerVisible = ref(false)

const selectedLog = ref({

  startTime: '',

  endTime: '',

  client: '',

  server: '',

  application: '',

  requestType: '',

  method: '',

  requestContent: '',

  status: '',

  responseTime: ''

})



// 复制内容弹窗
const copyDialogVisible = ref(false)

const copyFormat = ref('text')

const copyContent = ref('')



// 搜索调用日志

const searchLog = () => {
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



// 切换策略显示

const switchNameDisplay = () => {
  ElMessage.info('功能开发中...')
}



// 刷新调用日志

const refreshLog = () => {
  ElMessage.info('功能开发中...')
}



// 导出调用日志

const exportLog = () => {
  ElMessage.info('功能开发中...')
}



// 切换数据源功能
const toggleColumn = (column: string) => {
  ElMessage.info('功能开发中...')
}



// 处理调用日志行点击
const handleLogRowClick = (row: any) => {

  selectedLog.value = row

  logDrawerVisible.value = true

  }



// 复制调用日志

const copyLog = (row: any) => {

  selectedLog.value = row

  copyContent.value = JSON.stringify(row, null, 2)

  copyDialogVisible.value = true

  }



// 复制到剪贴板
const copyToClipboard = () => {
  navigator.clipboard.writeText(copyContent.value).then(() => {
    ElMessage.success('复制成功')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}



// 调用日志分页变化

const handleLogPageChange = (page: number) => {

  logCurrentPage.value = page

  }

</script>



<style scoped>

.app-log-content {

  padding: 20px;

}



.log-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.log-search {

  flex: 1;

}



.log-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.log-content {

  display: flex;

  gap: 20px;

}



.log-sidebar {

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



.log-main {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.log-chart {

  margin-bottom: 30px;

}



.chart-header h3 {

  margin: 0;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.log-chart-content {

  height: 200px;

}



.log-bar {

  background-color: #409eff;

  border-radius: 2px 2px 0 0;

}



.log-list {

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



.log-list .el-table {

  margin-bottom: 20px;

}



.log-list .el-table th {

  background-color: #f5f7fa;

}



.log-list .el-table td {

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



.log-drawer {

  padding: 20px;

}



.log-drawer h3 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.log-drawer h4 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.log-drawer h5 {

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



.copy-dialog {

  padding: 10px 0;

}



.dialog-footer {

  display: flex;

  justify-content: flex-end;

  gap: 10px;

}



@media (max-width: 1200px) {

  .log-content {

    flex-direction: column;

  }

  

  .log-sidebar {

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