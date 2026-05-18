<template>

  <div class="view-detail">

 <!-- TCP重传详情-->

    <div class="top-controls">

      <div class="controls-left">

        <el-select v-model="selectedView" placeholder="选择视图" style="width: 200px; margin-right: 10px;">

          <el-option v-for="view in views" :key="view.id" :label="view.name" :value="view.id" />

        </el-select>

        <el-select v-model="selectedRegion" placeholder="查询区域" style="width: 150px; margin-right: 10px;">

          <el-option label="全部服务管理" value="all" />

          <el-option label="区域1" value="region1" />

          <el-option label="区域2" value="region2" />

          <el-option label="区域3" value="region3" />

        </el-select>

        <el-date-picker

          v-model="dateRange"

          type="daterange"

          range-separator="至"

          start-placeholder="开始日期"

          end-placeholder="结束日期"

          style="width: 250px; margin-right: 10px;"

        />

        <el-select v-model="timeInterval" placeholder="时间间隔" style="width: 100px; margin-right: 10px;">

          <el-option label="1s" value="1s" />

          <el-option label="5s" value="5s" />

          <el-option label="10s" value="10s" />

          <el-option label="30s" value="30s" />

          <el-option label="1m" value="1m" />

          <el-option label="1h" value="1h" />

          <el-option label="1d" value="1d" />

        </el-select>

        <el-switch v-model="autoRefresh" active-text="自动" inactive-text="手动" style="margin-right: 10px;" />

        <el-button @click="refreshData">

          刷新

        </el-button>

      </div>

      <div class="controls-right">

        <el-button type="primary" @click="addSubView">

          新建子视图

        </el-button>

        <el-button @click="toggleFullscreen">

          全屏

        </el-button>

        <el-dropdown>

          <el-button>

            更多设置 <el-icon class="el-icon--right"><ArrowDown /></el-icon>

          </el-button>

          <template #dropdown>

            <el-dropdown-menu>

              <el-dropdown-item @click="switchDataSource">切换数据库源</el-dropdown-item>

              <el-dropdown-item @click="manageVariables">管理变量</el-dropdown-item>

              <el-dropdown-item @click="toggleTipSync">开启Tip同步</el-dropdown-item>

              <el-dropdown-item @click="exportView">导出视图</el-dropdown-item>

              <el-dropdown-item v-if="viewDetail.type !== 'built-in'" @click="deleteView">删除视图</el-dropdown-item>

              <el-dropdown-item @click="switchFillMode">切换插值方式</el-dropdown-item>

              <el-dropdown-item @click="saveAsCopy">另存为副本</el-dropdown-item>

            </el-dropdown-menu>

          </template>

        </el-dropdown>

      </div>

    </div>

    

 <!-- 图表工具栏 -->

    <div class="filters">

      <el-form :inline="true" :model="filterForm" class="filter-form">

        <el-form-item label="客户端服务">

          <el-select v-model="filterForm.client" placeholder="选择客户端服务" style="width: 150px;">

            <el-option label="全部服务" value="all" />

            <el-option label="客户端服务" value="client1" />

            <el-option label="客户端服务" value="client2" />

            <el-option label="客户端服务" value="client3" />

          </el-select>

        </el-form-item>

        <el-form-item label="云服务器">

          <el-select v-model="filterForm.server" placeholder="选择云服务器" style="width: 150px;">

            <el-option label="全部请求调用链" value="all" />

            <el-option label="请求服务器" value="server1" />

            <el-option label="请求服务器" value="server2" />

            <el-option label="请求服务器" value="server3" />

          </el-select>

        </el-form-item>

        <el-form-item label="请求资源">

          <el-input v-model="filterForm.resource" placeholder="输入请求资源" style="width: 200px;"></el-input>

        </el-form-item>

        <el-form-item label="响应状态筛选">

          <el-select v-model="filterForm.status" placeholder="选择响应状态筛选" style="width: 150px;">

            <el-option label="全部服务" value="all" />

            <el-option label="200" value="200" />

            <el-option label="404" value="404" />

            <el-option label="500" value="500" />

          </el-select>

        </el-form-item>

        <el-form-item>

          <el-button type="primary" @click="applyFilters">

            应用筛选

          </el-button>

          <el-button @click="resetFilters">

            显示

          </el-button>

        </el-form-item>

      </el-form>

    </div>

    

 <!-- 图表布局 -->

    <div class="charts-grid">

 <!-- 大数字卡 -->

      <div class="chart-item card-item">

        <div class="chart-header">

          <span class="chart-name">总请求成功率</span>

        </div>

        <div class="chart-content">

          <div class="card-value">1,234,567</div>

          <div class="card-trend">+12.5% 较上周</div>

        </div>

      </div>

      <div class="chart-item card-item">

        <div class="chart-header">

          <span class="chart-name">平均响应时间详情</span>

        </div>

        <div class="chart-content">

          <div class="card-value">123ms</div>

          <div class="card-trend">-5.2% 较上周</div>

        </div>

      </div>

      <div class="chart-item card-item">

        <div class="chart-header">

          <span class="chart-name">错误率</span>

        </div>

        <div class="chart-content">

          <div class="card-value">0.8%</div>

          <div class="card-trend">+0.2% 较上周</div>

        </div>

      </div>

      <div class="chart-item card-item">

        <div class="chart-header">

          <span class="chart-name">系统监控指标</span>

        </div>

        <div class="chart-content">

          <div class="card-value">567</div>

          <div class="card-trend">+8.3% 较上周</div>

        </div>

      </div>

      

 <!-- 柱状图-->

      <div class="chart-item line-item">

        <div class="chart-header">

          <span class="chart-name">响应时间分布</span>

          <div class="chart-actions">

            <el-button size="small" @click="editChart(1)">

              编辑

            </el-button>

            <ChartSettings 

              :chart-name="'响应时间分布'"

              :data-source="'DeepFlow'"

              @edit="editChart(1)"

              @add="handleAddToView"

              @download="handleDownloadCSV"

              @api="handleViewAPI"

              @switch-type="handleSwitchChartType"

            />

          </div>

        </div>

        <div class="chart-content">

          <div class="chart-placeholder">

 <!-- 此处放置放置实际的拓扑图 -->

            <p>检查所有容器和节点的详细信息和配置</p>

          </div>

        </div>

      </div>

      

 <!-- 饼图-->

      <div class="chart-item bar-item">

        <div class="chart-header">

          <span class="chart-name">资源池概览</span>

          <div class="chart-actions">

            <el-button size="small" @click="editChart(2)">

              编辑

            </el-button>

          </div>

        </div>

        <div class="chart-content">

          <div class="chart-placeholder">

 <!-- 此处放置放置实际的服务调用链 -->

            <p>当前视图支持多种图表类型</p>

          </div>

        </div>

      </div>

      

 <!-- 流量拓扑 -->

      <div class="chart-item topology-item">

        <div class="chart-header">

          <span class="chart-name">流量拓扑</span>

          <div class="chart-actions">

            <el-button size="small" @click="editChart(3)">

              编辑

            </el-button>

          </div>

        </div>

        <div class="chart-content">

          <div class="chart-placeholder">

 <!-- 此处放置放置实际的容器组件 -->

            <p>流量拓扑图</p>

          </div>

        </div>

      </div>

      

 <!-- 表格 -->

      <div class="chart-item table-item">

        <div class="chart-header">

          <span class="chart-name">服务列表</span>

          <div class="chart-actions">

            <el-button size="small" @click="editChart(4)">

              编辑

            </el-button>

          </div>

        </div>

        <div class="chart-content">

          <el-table :data="serviceList" style="width: 100%">

            <el-table-column prop="name" label="服务名称" width="150" />

            <el-table-column prop="requests" label="请求数量" width="100" />

            <el-table-column prop="responseTime" label="分组聚合详情" width="100" />

            <el-table-column prop="errorRate" label="错误率" width="100" />

            <el-table-column prop="status" label="状态" width="100" />

          </el-table>

        </div>

      </div>

    </div>

    

 <!-- 请求日志详情表格 -->

    <div class="logs-section">

      <div class="logs-header">

        <h3>请求日志详情</h3>

        <div class="logs-actions">

          <el-button @click="showColumnSelection">

            列选择

          </el-button>

          <el-button @click="exportLogs">

            导出数据库

          </el-button>

        </div>

      </div>

      <el-table :data="logsList" style="width: 100%">

        <el-table-column prop="time" label="请求时间" width="180" />

        <el-table-column prop="client" label="客户端服务" width="150" />

        <el-table-column prop="server" label="请求服务器" width="150" />

        <el-table-column prop="resource" label="请求资源" />

        <el-table-column prop="method" label="请求方法" width="100" />

        <el-table-column prop="status" label="响应状态筛选" width="100" />

        <el-table-column prop="responseTime" label="分组聚合详情" width="100" />

        <el-table-column label="操作" width="100" fixed="right">

          <template #default="scope">

            <el-button size="small" @click="drillDown(scope.row)">

              下载

            </el-button>

          </template>

        </el-table-column>

      </el-table>

      <div class="logs-pagination">

        <el-pagination

          @size-change="handleLogsSizeChange"

          @current-change="handleLogsCurrentChange"

          :current-page="logsPagination.current"

          :page-sizes="[10, 20, 50, 100]"

          :page-size="logsPagination.size"

          layout="total, sizes, prev, pager, next, jumper"

          :total="logsPagination.total"

        />

      </div>

    </div>

    

 <!-- 新建子图对话框 -->

    <el-dialog

      v-model="addSubViewVisible"

      title="新建子视图"

      width="600px"

    >

      <div class="add-subview-content">

        <h4>选择图表类型</h4>

        <div class="chart-types">

          <el-button @click="selectChartType('line')">柱状图</el-button>

          <el-button @click="selectChartType('bar')">饼图</el-button>

          <el-button @click="selectChartType('topology')">拓扑</el-button>

          <el-button @click="selectChartType('table')">表格</el-button>

          <el-button @click="selectChartType('group')">分组依据</el-button>

        </div>

      </div>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="addSubViewVisible = false">取消</el-button>

          <el-button type="primary" @click="confirmAddSubView">确定</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 编辑视图对话框-->

    <el-dialog

      v-model="editDialogVisible"

      title="编辑视图"

      width="600px"

    >

      <el-form :model="editForm" :rules="editRules" ref="editFormRef">

        <el-form-item label="视图名称" prop="name">

          <el-input v-model="editForm.name" placeholder="请输入视图名称" />

        </el-form-item>

        <el-form-item label="响应时间" prop="description">

          <el-input

            v-model="editForm.description"

            type="textarea"

            placeholder="请输入描述信息"

            :rows="3"

          ></el-input>

        </el-form-item>

        <el-form-item label="数据库来源" prop="dataSource">

          <el-select v-model="editForm.dataSource" placeholder="请选择数据源">

            <el-option label="Prometheus" value="prometheus" />

            <el-option label="Grafana" value="grafana" />

            <el-option label="DeepFlow" value="deepflow" />

          </el-select>

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="editDialogVisible = false">取消</el-button>

          <el-button type="primary" @click="submitEditForm">确定</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 删除确认对话框 -->

    <el-dialog

      v-model="deleteDialogVisible"

      title="删除确认"

      width="400px"

    >

      <p>确定要删除这个视图吗？</p>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="deleteDialogVisible = false">取消</el-button>

          <el-button type="danger" @click="confirmDelete">确认删除</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive, onMounted } from 'vue'

import { useRouter, useRoute } from 'vue-router'

import { ArrowDown } from '@element-plus/icons-vue'

import { ElMessage } from 'element-plus'

import ChartSettings from '../components/ChartSettings.vue'



const router = useRouter()

const route = useRoute()



// 视图列表

const views = ref([

  { id: 1, name: '系统概览' },

  { id: 2, name: '网络流量监控' },

  { id: 3, name: '应用性能监控' },

  { id: 4, name: '系统数据概览' },

  { id: 5, name: '应用最终指标' }

])



// 选中的视图

const selectedView = ref(1)



// 选中的区域

const selectedRegion = ref('all')



// 时间范围选择

const dateRange = ref([])



// 时间间隔

const timeInterval = ref('10s')



// 自动保存参数

const autoRefresh = ref(true)



// 筛选表单

const filterForm = reactive({

  client: 'all',

  server: 'all',

  resource: '',

  status: 'all'

})



// 系统整体运行状态

const viewDetail = ref({

  id: 1,

  name: '系统概览',

  description: '系统整体运行状态',

  dataSource: 'DeepFlow',

  createTime: '2023-09-01 10:00:00',

  updateTime: '2023-09-01 10:00:00',

  creator: 'admin',

  type: 'custom'

})



// 服务列表

const serviceList = ref([

  { id: 1, name: '服务1', requests: 12345, responseTime: '123ms', errorRate: '0.5%', status: '正常' },

  { id: 2, name: '服务2', requests: 9876, responseTime: '98ms', errorRate: '0.2%', status: '正常' },

  { id: 3, name: '服务3', requests: 5432, responseTime: '234ms', errorRate: '1.2%', status: '警告' },

  { id: 4, name: '服务4', requests: 7890, responseTime: '156ms', errorRate: '0.8%', status: '正常' }

])



// 日志列表

const logsList = ref([

  { id: 1, time: '2023-09-01 10:00:00', client: '客户端服务', server: '服务端服务', resource: '/api/user', method: 'GET', status: '200', responseTime: '123ms' },

  { id: 2, time: '2023-09-01 10:00:01', client: '客户端服务', server: '服务端服务', resource: '/api/order', method: 'POST', status: '200', responseTime: '98ms' },

  { id: 3, time: '2023-09-01 10:00:02', client: '客户端服务', server: '服务端服务', resource: '/api/product', method: 'GET', status: '404', responseTime: '45ms' },

  { id: 4, time: '2023-09-01 10:00:03', client: '客户端服务', server: '服务端服务', resource: '/api/payment', method: 'POST', status: '500', responseTime: '234ms' },

  { id: 5, time: '2023-09-01 10:00:04', client: '客户端服务', server: '服务端服务', resource: '/api/user', method: 'GET', status: '200', responseTime: '156ms' }

])



// 日志分页

const logsPagination = reactive({

  current: 1,

  size: 10,

  total: 100

})



// 对话框

const addSubViewVisible = ref(false)

const editDialogVisible = ref(false)

const deleteDialogVisible = ref(false)



// 编辑表单

const editForm = reactive({

  name: '',

  description: '',

  dataSource: ''

})



const editFormRef = ref()



// 表单验证规则

const editRules = reactive({

  name: [

    { required: true, message: '请输入视图名称', trigger: 'blur' },

    { min: 1, max: 50, message: '长度必须在1到50个字符之间', trigger: 'blur' }

  ],

  description: [

    { max: 200, message: '长度不能超过200个字符', trigger: 'blur' }

  ],

  dataSource: [

    { required: true, message: '请选择数据源', trigger: 'change' }

  ]

})



// 路由参数初始化

onMounted(() => {

 // 从路由参数获取视图ID

  const id = route.query.id

  if (id) {

 // 模板分页加载视图模板详情

    }

})



// 刷新数据

const refreshData = () => {

 // 根据参数初始化视图数据

}



// 新建子视图

const addSubView = () => {

  addSubViewVisible.value = true

}



// 选择图表类型

const selectChartType = (type: string) => {

 // 实现选择图表类型功能

}



// 确认删除当前视图
const confirmAddSubView = () => {

 // 实现添加子视图功能

  addSubViewVisible.value = false

}



// 编辑图表

const editChart = (id: number) => {

  router.push(`/views/add-chart?id=${id}`)

}



// 切换全屏

const toggleFullscreen = () => {

 // // 切换数据源功能

}



// 切换数据库源

const switchDataSource = () => {

 // 根据条件筛选并渲染拓扑图

}



// 管理变量

const manageVariables = () => {

 // 根据时间范围更新数据

}



// 开启Tip同步

const toggleTipSync = () => {

 // 实现显示/隐藏IP功能

}



// 导出视图

const exportView = () => {

 // 实现导出视图功能

}



// 删除视图

const deleteView = () => {

  deleteDialogVisible.value = true

}



// 切换插值方式

const switchFillMode = () => {

 // // 切换插值方式功能

}



// 另存为副本

const saveAsCopy = () => {

 // 根据条件保存为副本

}



// 应用筛选

const applyFilters = () => {

 // 根据时间间隔刷新数据

}



// 重置筛选

const resetFilters = () => {

  filterForm.client = 'all'

  filterForm.server = 'all'

  filterForm.resource = ''

  filterForm.status = 'all'

 // 根据条件导出数据

}



// 显示列选择

const showColumnSelection = () => {

 // // 显示列选择功能

}



// 导出日志

const exportLogs = () => {

 // 实现导出时间序列功能

}



// 下载

const drillDown = (row: any) => {

 // 实现下载数据功能

}



// 日志分页

const handleLogsSizeChange = (size: number) => {

  logsPagination.size = size

 // 根据条件确认相关事件

}



const handleLogsCurrentChange = (current: number) => {

  logsPagination.current = current

 // 根据条件确认相关事件

}



// 确认删除操作

const confirmDelete = () => {

 // 根据条件刷新视图数据

  deleteDialogVisible.value = false

  router.push('/views/list')

}



// 提交编辑表单

const submitEditForm = async () => {

  if (!editFormRef.value) return

  try {

    await editFormRef.value.validate()

 // 实现提交保存功能

    viewDetail.value.name = editForm.name

    viewDetail.value.description = editForm.description

    viewDetail.value.dataSource = editForm.dataSource

    editDialogVisible.value = false

  } catch (e) {

    console.error('表单验证失败', e)

  }

}



// 处理添加到视图

const handleAddToView = (form: any) => {

 // 实现应用拓扑功能

 // 显示名称Toast提示信息

  ElMessage.success('图表已成功添加到视图')

}



// 处理下载CSV

const handleDownloadCSV = () => {

 // 实现下载CSV功能

}



// 查看API

const handleViewAPI = () => {

 // 根据条件查看API功能

}



// 处理切换图表类型

const handleSwitchChartType = () => {

 // 实现切换图表类型功能

}

</script>



<style scoped>

.view-detail {

  background-color: white;

  border-radius: 4px;

  padding: 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  height: 100%;

  display: flex;

  flex-direction: column;

  gap: 24px;

}



.top-controls {

  display: flex;

  justify-content: space-between;

  align-items: center;

  padding-bottom: 16px;

  border-bottom: 1px solid #e4e7ed;

  flex-wrap: wrap;

  gap: 10px;

}



.controls-left {

  display: flex;

  align-items: center;

  gap: 10px;

  flex-wrap: wrap;

}



.controls-right {

  display: flex;

  align-items: center;

  gap: 10px;

}



.filters {

  padding: 16px 0;

  border-bottom: 1px solid #e4e7ed;

}



.filter-form {

  display: flex;

  align-items: center;

  gap: 10px;

  flex-wrap: wrap;

}



.charts-grid {

  display: grid;

  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));

  gap: 16px;

  flex: 1;

  overflow: auto;

}



.chart-item {

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  overflow: hidden;

  background-color: white;

}



.card-item {

  grid-column: span 1;

}



.line-item {

  grid-column: span 2;

  grid-row: span 1;

}



.bar-item {

  grid-column: span 2;

  grid-row: span 1;

}



.topology-item {

  grid-column: span 2;

  grid-row: span 2;

}



.table-item {

  grid-column: span 2;

  grid-row: span 1;

}



.chart-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  padding: 12px 16px;

  background-color: #f5f7fa;

  border-bottom: 1px solid #e4e7ed;

}



.chart-name {

  font-weight: bold;

  color: #303133;

}



.chart-actions {

  display: flex;

  gap: 8px;

}



.chart-content {

  padding: 16px;

  background-color: white;

}



.card-value {

  font-size: 24px;

  font-weight: bold;

  color: #303133;

  margin-bottom: 8px;

}



.card-trend {

  font-size: 14px;

  color: #67c23a;

}



.chart-placeholder {

  height: 100%;

  min-height: 200px;

  display: flex;

  align-items: center;

  justify-content: center;

  background-color: #f9f9f9;

  border-radius: 4px;

  color: #909399;

}



.logs-section {

  padding: 16px 0;

  border-top: 1px solid #e4e7ed;

}



.logs-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 16px;

}



.logs-header h3 {

  margin: 0;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.logs-actions {

  display: flex;

  gap: 10px;

}



.logs-pagination {

  padding-top: 16px;

  display: flex;

  justify-content: flex-end;

}



.add-subview-content {

  padding: 20px 0;

}



.chart-types {

  display: flex;

  gap: 10px;

  flex-wrap: wrap;

  margin-top: 20px;

}



.dialog-footer {

  display: flex;

  justify-content: flex-end;

  gap: 10px;

}



:deep(.el-button--primary) {

  background-color: #1677FF;

  border-color: #1677FF;

}



:deep(.el-button--danger) {

  background-color: #FF4D4F;

  border-color: #FF4D4F;

}



@media (max-width: 1200px) {

  .line-item,

  .bar-item,

  .topology-item,

  .table-item {

    grid-column: span 1;

  }

}

</style>