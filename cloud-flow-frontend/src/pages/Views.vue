<template>
  <div class="views-container">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">首页</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>视图管理</el-breadcrumb-item>
      </el-breadcrumb>
      <div class="header-actions">
        <h2>视图管理</h2>
        <div class="header-buttons">
          <el-button @click="importView">
            <el-icon><Upload /></el-icon> 导入视图
          </el-button>
          <el-button type="primary" @click="createView">
            <el-icon><Plus /></el-icon> 新建视图
          </el-button>
        </div>
      </div>
    </div>

    <!-- 搜索筛选区域 -->
    <div class="filter-section">
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="视图名称">
          <el-input v-model="searchForm.keyword" placeholder="输入视图名称搜索" style="width: 220px;" clearable />
        </el-form-item>
        <el-form-item label="视图类型">
          <el-select v-model="searchForm.type" placeholder="全部类型" style="width: 140px;" clearable>
            <el-option label="内置视图" value="built-in" />
            <el-option label="自定义视图" value="custom" />
            <el-option label="共享视图" value="shared" />
          </el-select>
        </el-form-item>
        <el-form-item label="创建人">
          <el-select v-model="searchForm.creator" placeholder="全部" style="width: 140px;" clearable>
            <el-option label="admin" value="admin" />
            <el-option label="operator" value="operator" />
            <el-option label="developer" value="developer" />
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

    <!-- 视图列表 -->
    <div class="table-section">
      <el-table :data="viewList" style="width: 100%" stripe>
        <el-table-column prop="name" label="视图名称" min-width="160">
          <template #default="scope">
            <el-button type="primary" link @click="openView(scope.row)">{{ scope.row.name }}</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="100" align="center">
          <template #default="scope">
            <el-tag :type="scope.row.type === '内置视图' ? 'primary' : scope.row.type === '自定义视图' ? 'success' : 'warning'" size="small">
              {{ scope.row.type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="chartCount" label="图表数量" width="100" align="center" />
        <el-table-column prop="creator" label="创建人" width="100" align="center" />
        <el-table-column prop="updateTime" label="更新时间" width="170" sortable />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="scope">
            <el-button size="small" link type="primary" @click="openView(scope.row)">打开</el-button>
            <el-button size="small" link type="primary" @click="editView(scope.row)">编辑</el-button>
            <el-button size="small" link type="primary" @click="duplicateView(scope.row)">复制</el-button>
            <el-button size="small" link type="primary" @click="exportView(scope.row)">导出</el-button>
            <el-button size="small" link type="danger" @click="deleteView(scope.row)" :disabled="scope.row.type === '内置视图'">
              删除
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

    <!-- 新建视图对话框 -->
    <el-dialog v-model="dialogVisible" title="新建视图" width="600px" destroy-on-close>
      <el-form :model="viewForm" :rules="viewRules" ref="viewFormRef" label-width="100px">
        <el-form-item label="视图名称" prop="name">
          <el-input v-model="viewForm.name" placeholder="请输入视图名称" />
        </el-form-item>
        <el-form-item label="视图描述" prop="description">
          <el-input v-model="viewForm.description" type="textarea" placeholder="请输入视图描述" :rows="3" />
        </el-form-item>
        <el-form-item label="视图类型" prop="type">
          <el-radio-group v-model="viewForm.type">
            <el-radio label="custom">自定义视图</el-radio>
            <el-radio label="shared">共享视图</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="基础布局">
          <el-select v-model="viewForm.layout" placeholder="请选择布局模板" style="width: 100%;">
            <el-option label="空白布局" value="blank" />
            <el-option label="服务监控布局" value="service-monitor" />
            <el-option label="基础设施布局" value="infra-monitor" />
            <el-option label="网络分析布局" value="network-analysis" />
            <el-option label="业务分析布局" value="business-analysis" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitForm">创建</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { Search, Plus, Upload } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()

// 搜索表单
const searchForm = ref({
  keyword: '',
  type: '',
  creator: ''
})

// 视图列表数据
const viewList = ref([
  {
    id: 1,
    name: '服务性能总览',
    type: '内置视图',
    description: '展示所有服务的核心性能指标，包括延迟、吞吐量、错误率等',
    chartCount: 12,
    creator: 'admin',
    updateTime: '2024-01-15 10:00:00'
  },
  {
    id: 2,
    name: '应用拓扑分析',
    type: '内置视图',
    description: '应用间的调用拓扑关系及关键指标展示',
    chartCount: 8,
    creator: 'admin',
    updateTime: '2024-01-14 16:30:00'
  },
  {
    id: 3,
    name: '基础设施监控',
    type: '内置视图',
    description: '主机、容器、进程等基础设施资源的监控视图',
    chartCount: 15,
    creator: 'admin',
    updateTime: '2024-01-15 09:45:00'
  },
  {
    id: 4,
    name: '网络流量分析',
    type: '内置视图',
    description: '网络流量的分布、趋势和异常分析视图',
    chartCount: 10,
    creator: 'admin',
    updateTime: '2024-01-13 14:20:00'
  },
  {
    id: 5,
    name: '告警统计汇总',
    type: '内置视图',
    description: '各类告警的统计分析和趋势展示',
    chartCount: 6,
    creator: 'admin',
    updateTime: '2024-01-15 08:00:00'
  },
  {
    id: 6,
    name: '电商平台监控',
    type: '自定义视图',
    description: '电商核心业务链路的性能监控和告警视图',
    chartCount: 18,
    creator: 'operator',
    updateTime: '2024-01-15 10:15:00'
  },
  {
    id: 7,
    name: '数据库性能看板',
    type: '共享视图',
    description: 'MySQL/Redis等数据库的性能指标和慢查询分析',
    chartCount: 9,
    creator: 'developer',
    updateTime: '2024-01-14 11:30:00'
  },
  {
    id: 8,
    name: 'Kubernetes集群监控',
    type: '自定义视图',
    description: 'K8s集群节点、Pod、Deployment等资源监控',
    chartCount: 14,
    creator: 'operator',
    updateTime: '2024-01-15 07:30:00'
  },
  {
    id: 9,
    name: 'API网关监控',
    type: '共享视图',
    description: 'API网关的请求量、延迟、错误率等核心指标',
    chartCount: 7,
    creator: 'developer',
    updateTime: '2024-01-12 16:45:00'
  },
  {
    id: 10,
    name: '容器资源使用',
    type: '内置视图',
    description: '容器CPU、内存、网络、磁盘等资源使用情况',
    chartCount: 11,
    creator: 'admin',
    updateTime: '2024-01-15 09:00:00'
  }
])

// 分页
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(10)

// 对话框
const dialogVisible = ref(false)
const viewFormRef = ref()

// 表单
const viewForm = reactive({
  name: '',
  description: '',
  type: 'custom',
  layout: 'blank'
})

// 表单验证规则
const viewRules = reactive({
  name: [
    { required: true, message: '请输入视图名称', trigger: 'blur' },
    { min: 1, max: 50, message: '长度在 1 到 50 个字符', trigger: 'blur' }
  ],
  description: [
    { max: 200, message: '长度不能超过 200 个字符', trigger: 'blur' }
  ],
  type: [
    { required: true, message: '请选择视图类型', trigger: 'change' }
  ]
})

// 搜索
const handleSearch = () => {
  ElMessage.info('搜索功能开发中...')
}

// 重置
const handleReset = () => {
  searchForm.value = { keyword: '', type: '', creator: '' }
}

// 分页
const handlePageChange = (page: number) => {
  currentPage.value = page
}

// 打开视图
const openView = (row: any) => {
  router.push(`/views/detail/${row.id}`)
}

// 编辑视图
const editView = (row: any) => {
  router.push(`/views/edit/${row.id}`)
}

// 复制视图
const duplicateView = (row: any) => {
  ElMessageBox.confirm(`确定要复制视图 "${row.name}" 吗？`, '复制确认', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'info'
  }).then(() => {
    ElMessage.success('视图复制成功')
  }).catch(() => {})
}

// 导出视图
const exportView = (row: any) => {
  ElMessage.success(`正在导出视图: ${row.name}`)
}

// 删除视图
const deleteView = (row: any) => {
  ElMessageBox.confirm(`确定要删除视图 "${row.name}" 吗？删除后不可恢复。`, '删除确认', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    ElMessage.success('删除成功')
  }).catch(() => {})
}

// 新建视图
const createView = () => {
  viewForm.name = ''
  viewForm.description = ''
  viewForm.type = 'custom'
  viewForm.layout = 'blank'
  dialogVisible.value = true
}

// 导入视图
const importView = () => {
  ElMessage.info('导入视图功能开发中...')
}

// 提交表单
const submitForm = async () => {
  if (!viewFormRef.value) return
  try {
    await viewFormRef.value.validate()
    ElMessage.success('视图创建成功')
    dialogVisible.value = false
  } catch (e) {
    console.error('表单验证失败', e)
  }
}
</script>

<style scoped>
.views-container {
  padding: 20px;
  height: 100vh;
  background-color: #f5f7fa;
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

.header-buttons {
  display: flex;
  gap: 10px;
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
