<template>
  <div class="report-strategy-page">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">首页</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>报表管理</el-breadcrumb-item>
        <el-breadcrumb-item>报表策略</el-breadcrumb-item>
      </el-breadcrumb>
      <div class="header-actions">
        <h2>报表策略</h2>
        <el-button type="primary" @click="createStrategy">
          <el-icon><Plus /></el-icon> 新建策略
        </el-button>
      </div>
    </div>

    <!-- 搜索筛选区域 -->
    <div class="filter-section">
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="策略名称">
          <el-input v-model="searchForm.keyword" placeholder="输入策略名称搜索" style="width: 220px;" clearable />
        </el-form-item>
        <el-form-item label="报表类型">
          <el-select v-model="searchForm.type" placeholder="全部类型" style="width: 140px;" clearable>
            <el-option label="日报" value="daily" />
            <el-option label="周报" value="weekly" />
            <el-option label="月报" value="monthly" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="全部状态" style="width: 140px;" clearable>
            <el-option label="已启用" value="enabled" />
            <el-option label="已停用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">
            <el-icon><Search /></el-icon> 搜索
          </el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 策略列表 -->
    <div class="table-section">
      <el-table :data="strategyList" style="width: 100%" stripe>
        <el-table-column prop="name" label="策略名称" min-width="150">
          <template #default="scope">
            <el-button type="primary" link @click="viewStrategy(scope.row)">
              {{ scope.row.name }}
              <span class="report-count">(已生成{{ scope.row.reportCount }}份)</span>
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="80" align="center">
          <template #default="scope">
            <el-tag :type="scope.row.type === '日报' ? 'primary' : scope.row.type === '周报' ? 'success' : 'warning'" size="small">
              {{ scope.row.type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cycle" label="执行周期" width="120" align="center" />
        <el-table-column prop="viewName" label="关联视图" min-width="130">
          <template #default="scope">
            <el-button link @click="viewView(scope.row)">{{ scope.row.viewName }}</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="format" label="输出格式" width="100" align="center" />
        <el-table-column prop="recipient" label="推送邮箱" min-width="180" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="80" align="center">
          <template #default="scope">
            <el-switch
              v-model="scope.row.enabled"
              @change="toggleStatus(scope.row)"
              active-text="启用"
              inactive-text="停用"
            />
          </template>
        </el-table-column>
        <el-table-column prop="createTime" label="创建时间" width="170" sortable />
        <el-table-column prop="updateTime" label="更新时间" width="170" sortable />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="scope">
            <el-button size="small" link type="primary" @click="editStrategy(scope.row)">
              <el-icon><Edit /></el-icon> 编辑
            </el-button>
            <el-button size="small" link type="primary" @click="executeNow(scope.row)">
              <el-icon><VideoPlay /></el-icon> 立即执行
            </el-button>
            <el-button size="small" link type="danger" @click="deleteStrategy(scope.row)">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
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
    </div>

    <!-- 新建/编辑策略对话框 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="650px" destroy-on-close>
      <el-form :model="strategyForm" :rules="strategyRules" ref="strategyFormRef" label-width="100px">
        <el-form-item label="策略名称" prop="name">
          <el-input v-model="strategyForm.name" placeholder="请输入策略名称" />
        </el-form-item>
        <el-form-item label="报表类型" prop="type">
          <el-select v-model="strategyForm.type" placeholder="请选择报表类型" style="width: 100%;">
            <el-option label="日报" value="日报" />
            <el-option label="周报" value="周报" />
            <el-option label="月报" value="月报" />
          </el-select>
        </el-form-item>
        <el-form-item label="执行周期" prop="cycle">
          <el-select v-model="strategyForm.cycle" placeholder="请选择执行周期" style="width: 100%;">
            <el-option label="每天" value="每天" />
            <el-option label="每周一" value="每周一" />
            <el-option label="每月1日" value="每月1日" />
          </el-select>
        </el-form-item>
        <el-form-item label="关联视图" prop="viewName">
          <el-select v-model="strategyForm.viewName" placeholder="请选择关联视图" style="width: 100%;">
            <el-option label="服务性能总览" value="服务性能总览" />
            <el-option label="应用拓扑分析" value="应用拓扑分析" />
            <el-option label="基础设施监控" value="基础设施监控" />
            <el-option label="网络流量分析" value="网络流量分析" />
            <el-option label="告警统计汇总" value="告警统计汇总" />
          </el-select>
        </el-form-item>
        <el-form-item label="输出格式" prop="format">
          <el-checkbox-group v-model="strategyForm.format">
            <el-checkbox label="PDF" />
            <el-checkbox label="Excel" />
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="推送邮箱" prop="recipient">
          <el-input v-model="strategyForm.recipient" placeholder="多个邮箱用逗号分隔" />
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="strategyForm.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitForm">确定</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { Search, Plus, Edit, Delete, VideoPlay } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

// 搜索表单
const searchForm = ref({
  keyword: '',
  type: '',
  status: ''
})

// 策略列表数据
const strategyList = ref([
  {
    id: 1,
    name: '每日服务性能报表',
    type: '日报',
    cycle: '每天',
    viewName: '服务性能总览',
    format: 'PDF',
    recipient: 'dev-team@example.com, ops@example.com',
    enabled: true,
    reportCount: 45,
    createTime: '2023-07-11 11:42:39',
    updateTime: '2024-01-10 08:00:00'
  },
  {
    id: 2,
    name: '每周拓扑分析报表',
    type: '周报',
    cycle: '每周一',
    viewName: '应用拓扑分析',
    format: 'PDF',
    recipient: 'arch@example.com',
    enabled: true,
    reportCount: 22,
    createTime: '2023-08-09 15:27:43',
    updateTime: '2024-01-08 09:00:00'
  },
  {
    id: 3,
    name: '每日基础设施报表',
    type: '日报',
    cycle: '每天',
    viewName: '基础设施监控',
    format: 'Excel',
    recipient: 'infra-team@example.com',
    enabled: true,
    reportCount: 120,
    createTime: '2023-07-08 19:49:48',
    updateTime: '2024-01-15 00:10:00'
  },
  {
    id: 4,
    name: '网络流量周报表',
    type: '周报',
    cycle: '每周一',
    viewName: '网络流量分析',
    format: 'PDF',
    recipient: 'network@example.com',
    enabled: false,
    reportCount: 15,
    createTime: '2023-06-25 14:44:44',
    updateTime: '2023-12-30 11:35:49'
  },
  {
    id: 5,
    name: '月度告警统计报表',
    type: '月报',
    cycle: '每月1日',
    viewName: '告警统计汇总',
    format: 'PDF, Excel',
    recipient: 'management@example.com, ops@example.com',
    enabled: true,
    reportCount: 6,
    createTime: '2023-05-18 18:43:36',
    updateTime: '2024-01-01 03:00:00'
  },
  {
    id: 6,
    name: '容器资源日报',
    type: '日报',
    cycle: '每天',
    viewName: '容器资源使用',
    format: 'PDF',
    recipient: 'k8s-team@example.com',
    enabled: false,
    reportCount: 30,
    createTime: '2023-05-09 17:16:15',
    updateTime: '2023-11-20 08:30:00'
  },
  {
    id: 7,
    name: '数据库性能周报',
    type: '周报',
    cycle: '每周一',
    viewName: '数据库性能监控',
    format: 'PDF',
    recipient: 'dba@example.com',
    enabled: true,
    reportCount: 18,
    createTime: '2023-09-15 10:00:00',
    updateTime: '2024-01-15 04:15:22'
  }
])

// 分页
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(7)

// 对话框
const dialogVisible = ref(false)
const dialogTitle = ref('新建报表策略')
const strategyFormRef = ref()

// 表单
const strategyForm = reactive({
  name: '',
  type: '',
  cycle: '',
  viewName: '',
  format: [] as string[],
  recipient: '',
  enabled: true
})

// 表单验证规则
const strategyRules = reactive({
  name: [{ required: true, message: '请输入策略名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择报表类型', trigger: 'change' }],
  cycle: [{ required: true, message: '请选择执行周期', trigger: 'change' }],
  viewName: [{ required: true, message: '请选择关联视图', trigger: 'change' }],
  recipient: [{ required: true, message: '请输入推送邮箱', trigger: 'blur' }]
})

// 搜索
const handleSearch = () => {
  ElMessage.info('搜索功能开发中...')
}

// 重置
const handleReset = () => {
  searchForm.value = { keyword: '', type: '', status: '' }
}

// 分页
const handlePageChange = (page: number) => {
  currentPage.value = page
}

// 查看策略
const viewStrategy = (row: any) => {
  ElMessage.info('查看策略详情功能开发中...')
}

// 查看视图
const viewView = (row: any) => {
  ElMessage.info('查看视图功能开发中...')
}

// 切换状态
const toggleStatus = (row: any) => {
  ElMessage.success(`策略 "${row.name}" 已${row.enabled ? '启用' : '停用'}`)
}

// 新建策略
const createStrategy = () => {
  dialogTitle.value = '新建报表策略'
  strategyForm.name = ''
  strategyForm.type = ''
  strategyForm.cycle = ''
  strategyForm.viewName = ''
  strategyForm.format = []
  strategyForm.recipient = ''
  strategyForm.enabled = true
  dialogVisible.value = true
}

// 编辑策略
const editStrategy = (row: any) => {
  dialogTitle.value = '编辑报表策略'
  strategyForm.name = row.name
  strategyForm.type = row.type
  strategyForm.cycle = row.cycle
  strategyForm.viewName = row.viewName
  strategyForm.format = row.format.split(', ').map((f: string) => f.trim())
  strategyForm.recipient = row.recipient
  strategyForm.enabled = row.enabled
  dialogVisible.value = true
}

// 立即执行
const executeNow = (row: any) => {
  ElMessageBox.confirm(`确定要立即执行策略 "${row.name}" 吗？`, '执行确认', {
    confirmButtonText: '确定执行',
    cancelButtonText: '取消',
    type: 'info'
  }).then(() => {
    ElMessage.success('策略已加入执行队列')
  }).catch(() => {})
}

// 删除策略
const deleteStrategy = (row: any) => {
  ElMessageBox.confirm(`确定要删除策略 "${row.name}" 吗？删除后不可恢复。`, '删除确认', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    ElMessage.success('删除成功')
  }).catch(() => {})
}

// 提交表单
const submitForm = async () => {
  if (!strategyFormRef.value) return
  try {
    await strategyFormRef.value.validate()
    ElMessage.success(dialogTitle.value === '新建报表策略' ? '创建成功' : '更新成功')
    dialogVisible.value = false
  } catch (e) {
    console.error('表单验证失败', e)
  }
}
</script>

<style scoped>
.report-strategy-page {
  padding: 20px;
  background-color: #f5f7fa;
  min-height: 100%;
}

.page-header {
  margin-bottom: 20px;
}

.header-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions h2 {
  margin: 0;
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

.filter-section {
  background-color: white;
  border-radius: 4px;
  padding: 16px 20px;
  margin-bottom: 16px;
}

.search-form {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: flex-end;
}

.table-section {
  background-color: white;
  border-radius: 4px;
  padding: 20px;
}

.report-count {
  color: #909399;
  font-size: 12px;
  font-weight: normal;
}

.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 20px;
}

.pagination-info {
  color: #909399;
  font-size: 14px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
