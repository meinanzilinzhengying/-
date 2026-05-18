<template>
  <div class="collector-management">
 <!-- 操作按钮 -->
    <div class="action-buttons mb-4">
      <el-button type="primary" @click="handleEnable">启用</el-button>
      <el-button type="warning" @click="handleDisable">禁用</el-button>
      <el-button type="info" @click="handleRegister">注册</el-button>
      <el-button @click="handleAddToGroup">加入采集器组</el-button>
      <el-button @click="handleExportCSV">导出 CSV</el-button>
    </div>

 <!-- 搜索和筛选 -->
    <div class="search-filter mb-4">
      <el-form :inline="true" :model="searchForm" class="demo-form-inline">
        <el-form-item label="采集器组">
          <el-select v-model="searchForm.group" placeholder="选择采集器组">
            <el-option label="全部" value="all" />
            <el-option label="default" value="default" />
            <el-option label="TB-Sandbox" value="TB-Sandbox" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-input v-model="searchForm.keyword" placeholder="搜索" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
        </el-form-item>
      </el-form>
    </div>

 <!-- 采集器列表 -->
    <el-table :data="collectors" style="width: 100%">
      <el-table-column type="selection" width="55" />
      <el-table-column prop="name" label="名称" min-width="180">
        <template #default="scope">
          <el-button  @click="viewCollectorDetail(scope.row)">{{ scope.row.name }}</el-button>
        </template>
      </el-table-column>
      <el-table-column prop="team" label="团队" width="120" />
      <el-table-column prop="group" label="组" width="120">
        <template #default="scope">
          <el-button  @click="viewGroupDetail(scope.row.group)">{{ scope.row.group }}</el-button>
        </template>
      </el-table-column>
      <el-table-column prop="type" label="类型" width="120" />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="scope">
          <el-tag :type="getStatusType(scope.row.status)">{{ scope.row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="version" label="软件版本" width="120" />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="scope">
          <el-button size="small" @click="toggleCollector(scope.row)">{{ scope.row.status === '运行' ? '禁用' : '启用' }}</el-button>
          <el-button size="small" type="danger" @click="deleteCollector(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

 <!-- 分页 -->
    <div class="pagination mt-4">
      <el-pagination
        background
        layout="prev, pager, next, jumper"
        :total="total"
        :page-size="pageSize"
        :current-page="currentPage"
        @current-change="handleCurrentChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { Warning } from '@element-plus/icons-vue'

const searchForm = ref({
  group: 'all',
  keyword: ''
})

const collectors = ref([
  {
    id: '1',
    name: 'sandbox-192.168.1.103-V3',
    team: 'Sandbox',
    group: 'TB-Sandbox',
    type: '容器-V',
    status: '运行',
    version: 'v1.0.0'
  }
])

const total = ref(100)
const pageSize = ref(20)
const currentPage = ref(1)

const handleSearch = () => {
}

const handleEnable = () => {
}

const handleDisable = () => {
}

const handleRegister = () => {
}

const handleAddToGroup = () => {
}

const handleExportCSV = () => {
}

const handleCurrentChange = (page: number) => {
  currentPage.value = page
}

const viewCollectorDetail = (row: any) => {
}

const viewGroupDetail = (group: string) => {
}

const toggleCollector = (row: any) => {
}

const deleteCollector = (row: any) => {
}

const getStatusType = (status: string) => {
  return status === '运行' ? 'success' : 'info'
}
</script>

<style scoped>
.collector-management {
  padding: 20px;
}
</style>