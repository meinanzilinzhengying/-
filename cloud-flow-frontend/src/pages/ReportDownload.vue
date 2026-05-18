<template>
  <div class="report-download-page">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">首页</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>报表管理</el-breadcrumb-item>
        <el-breadcrumb-item>报表下载</el-breadcrumb-item>
      </el-breadcrumb>
      <h2>报表下载</h2>
    </div>

    <!-- 搜索筛选区域 -->
    <div class="filter-section">
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="视图名称/调度策略">
          <el-input v-model="searchForm.keyword" placeholder="输入视图名称或调度策略" style="width: 280px;" clearable />
        </el-form-item>
        <el-form-item label="报表类型">
          <el-select v-model="searchForm.type" placeholder="全部类型" style="width: 140px;" clearable>
            <el-option label="日报" value="daily" />
            <el-option label="周报" value="weekly" />
            <el-option label="月报" value="monthly" />
          </el-select>
        </el-form-item>
        <el-form-item label="生成时间">
          <el-date-picker
            v-model="searchForm.dateRange"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            style="width: 280px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">
            <el-icon><Search /></el-icon> 搜索
          </el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 操作栏 -->
    <div class="action-bar">
      <el-button type="primary" @click="batchDownload">
        <el-icon><Download /></el-icon> 批量下载
      </el-button>
      <el-button type="danger" @click="batchDelete">
        <el-icon><Delete /></el-icon> 批量删除
      </el-button>
    </div>

    <!-- 报表下载列表 -->
    <div class="table-section">
      <el-table :data="reportList" style="width: 100%" stripe @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="50" />
        <el-table-column prop="viewName" label="视图名称" min-width="150">
          <template #default="scope">
            <el-button type="primary" link @click="viewReport(scope.row)">{{ scope.row.viewName }}</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="strategyName" label="调度策略" min-width="150" show-overflow-tooltip />
        <el-table-column prop="type" label="类型" width="80" align="center">
          <template #default="scope">
            <el-tag :type="scope.row.type === '日报' ? 'primary' : scope.row.type === '周报' ? 'success' : 'warning'" size="small">
              {{ scope.row.type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="format" label="格式" width="80" align="center" />
        <el-table-column prop="size" label="大小" width="100" align="center" />
        <el-table-column prop="startTime" label="数据开始时间" width="170" sortable />
        <el-table-column prop="endTime" label="数据结束时间" width="170" sortable />
        <el-table-column prop="createTime" label="生成时间" width="170" sortable />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="scope">
            <el-button size="small" type="primary" link @click="downloadReport(scope.row)">
              <el-icon><Download /></el-icon> 下载
            </el-button>
            <el-button size="small" type="danger" link @click="deleteReport(scope.row)">
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
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Search, Download, Delete } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

// 搜索表单
const searchForm = ref({
  keyword: '',
  type: '',
  dateRange: []
})

// 报表列表数据
const reportList = ref([
  {
    id: 1,
    viewName: '服务性能总览',
    strategyName: '每日服务性能报表',
    type: '日报',
    format: 'PDF',
    size: '2.3MB',
    startTime: '2024-01-14 00:00:00',
    endTime: '2024-01-15 00:00:00',
    createTime: '2024-01-15 00:05:23'
  },
  {
    id: 2,
    viewName: '应用拓扑分析',
    strategyName: '每周拓扑分析报表',
    type: '周报',
    format: 'PDF',
    size: '5.8MB',
    startTime: '2024-01-08 00:00:00',
    endTime: '2024-01-15 00:00:00',
    createTime: '2024-01-15 01:12:45'
  },
  {
    id: 3,
    viewName: '基础设施监控',
    strategyName: '每日基础设施报表',
    type: '日报',
    format: 'Excel',
    size: '1.5MB',
    startTime: '2024-01-14 00:00:00',
    endTime: '2024-01-15 00:00:00',
    createTime: '2024-01-15 00:08:11'
  },
  {
    id: 4,
    viewName: '网络流量分析',
    strategyName: '网络流量周报表',
    type: '周报',
    format: 'PDF',
    size: '4.2MB',
    startTime: '2024-01-08 00:00:00',
    endTime: '2024-01-15 00:00:00',
    createTime: '2024-01-15 02:30:00'
  },
  {
    id: 5,
    viewName: '告警统计汇总',
    strategyName: '月度告警统计',
    type: '月报',
    format: 'Excel',
    size: '8.1MB',
    startTime: '2023-12-01 00:00:00',
    endTime: '2024-01-01 00:00:00',
    createTime: '2024-01-01 03:00:00'
  },
  {
    id: 6,
    viewName: '容器资源使用',
    strategyName: '每日容器资源报表',
    type: '日报',
    format: 'PDF',
    size: '3.2MB',
    startTime: '2024-01-14 00:00:00',
    endTime: '2024-01-15 00:00:00',
    createTime: '2024-01-15 00:10:33'
  },
  {
    id: 7,
    viewName: '数据库性能监控',
    strategyName: '数据库性能周报',
    type: '周报',
    format: 'PDF',
    size: '6.5MB',
    startTime: '2024-01-08 00:00:00',
    endTime: '2024-01-15 00:00:00',
    createTime: '2024-01-15 04:15:22'
  },
  {
    id: 8,
    viewName: 'API调用统计',
    strategyName: '每日API统计报表',
    type: '日报',
    format: 'Excel',
    size: '1.8MB',
    startTime: '2024-01-14 00:00:00',
    endTime: '2024-01-15 00:00:00',
    createTime: '2024-01-15 00:06:18'
  }
])

// 分页
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(8)

// 多选
const selectedRows = ref<any[]>([])

// 搜索
const handleSearch = () => {
  ElMessage.info('搜索功能开发中...')
}

// 重置
const handleReset = () => {
  searchForm.value = { keyword: '', type: '', dateRange: [] }
}

// 分页
const handlePageChange = (page: number) => {
  currentPage.value = page
}

// 多选变化
const handleSelectionChange = (rows: any[]) => {
  selectedRows.value = rows
}

// 查看报表
const viewReport = (row: any) => {
  ElMessage.info('查看报表功能开发中...')
}

// 下载报表
const downloadReport = (row: any) => {
  ElMessage.success(`正在下载: ${row.viewName}.${row.format.toLowerCase()}`)
}

// 删除报表
const deleteReport = (row: any) => {
  ElMessageBox.confirm(`确定要删除报表 "${row.viewName}" 吗？`, '删除确认', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    ElMessage.success('删除成功')
  }).catch(() => {})
}

// 批量下载
const batchDownload = () => {
  if (selectedRows.value.length === 0) {
    ElMessage.warning('请先选择要下载的报表')
    return
  }
  ElMessage.success(`正在批量下载 ${selectedRows.value.length} 份报表`)
}

// 批量删除
const batchDelete = () => {
  if (selectedRows.value.length === 0) {
    ElMessage.warning('请先选择要删除的报表')
    return
  }
  ElMessageBox.confirm(`确定要删除选中的 ${selectedRows.value.length} 份报表吗？`, '批量删除确认', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    ElMessage.success('批量删除成功')
  }).catch(() => {})
}
</script>

<style scoped>
.report-download-page {
  padding: 20px;
  background-color: #f5f7fa;
  min-height: 100%;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 8px 0 0 0;
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

.action-bar {
  background-color: white;
  border-radius: 4px;
  padding: 12px 20px;
  margin-bottom: 16px;
  display: flex;
  gap: 10px;
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
</style>
