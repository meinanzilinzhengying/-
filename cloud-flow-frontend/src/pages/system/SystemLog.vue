<template>
  <div class="system-log">
 <!-- 标签页 -->
    <el-tabs v-model="activeLogTab">
      <el-tab-pane label="全部" name="all">
 <!-- 搜索 -->
        <div class="search-filter mb-4">
          <el-form :inline="true" :model="logSearchForm" class="demo-form-inline">
            <el-form-item label="用户">
              <el-input v-model="logSearchForm.username" placeholder="用户名" />
            </el-form-item>
            <el-form-item label="日志级别">
              <el-select v-model="logSearchForm.level" placeholder="日志级别">
                <el-option label="全部" value="all" />
                <el-option label="ERROR" value="ERROR" />
                <el-option label="WARN" value="WARN" />
                <el-option label="INFO" value="INFO" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="handleLogSearch">搜索</el-button>
            </el-form-item>
          </el-form>
        </div>

 <!-- 日志列表 -->
        <el-table :data="logs" style="width: 100%">
          <el-table-column prop="time" label="时间" width="180" />
          <el-table-column prop="user" label="用户" width="120" />
          <el-table-column prop="level" label="级别" width="100">
            <template #default="scope">
              <el-tag :type="getLogLevelType(scope.row.level)">{{ scope.row.level }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="module" label="模块" width="120" />
          <el-table-column prop="action" label="操作" width="150" />
          <el-table-column prop="detail" label="详情" min-width="300" />
        </el-table>

 <!-- 分页 -->
        <div class="pagination mt-4">
          <el-pagination
            background
            layout="prev, pager, next, jumper"
            :total="logTotal"
            :page-size="pageSize"
            :current-page="logCurrentPage"
            @current-change="handleLogCurrentChange"
          />
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const activeLogTab = ref('all')

const logSearchForm = ref({
  username: '',
  level: 'all'
})

const logs = ref([
  {
    time: '2026-04-23 10:00:00',
    user: 'admin',
    level: 'INFO',
    module: '用户管理',
    action: '登录',
    detail: '用户 admin 登录系统'
  },
  {
    time: '2026-04-23 10:05:00',
    user: 'admin',
    level: 'WARN',
    module: '采集器',
    action: '状态变更',
    detail: '采集器 sandbox-192.168.1.103 状态变更'
  }
])

const logTotal = ref(100)
const logCurrentPage = ref(1)
const pageSize = ref(20)

const handleLogSearch = () => {
}

const handleLogCurrentChange = (page: number) => {
  logCurrentPage.value = page
}

const getLogLevelType = (level: string) => {
  switch (level) {
    case 'ERROR': return 'danger'
    case 'WARN': return 'warning'
    case 'INFO': return 'success'
    default: return 'info'
  }
}
</script>

<style scoped>
.system-log {
  padding: 20px;
}
</style>