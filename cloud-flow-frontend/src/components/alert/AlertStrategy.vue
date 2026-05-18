<template>
  <el-card class="mb-4">
    <template #header>
      <div class="card-header">
        <h2>告警策略</h2>
        <div class="header-actions">
          <el-button type="primary" @click="createAlertStrategy">新建告警策略</el-button>
        </div>
      </div>
    </template>

    <!-- 搜索表单 -->
    <div class="search-filter mb-4">
      <el-form :inline="true" :model="searchForm" class="demo-form-inline">
        <el-form-item label="按名称搜索">
          <el-input v-model="searchForm.keyword" placeholder="按名称搜索" style="width: 300px;" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 告警策略列表 -->
    <el-table :data="alertStrategies" style="width: 100%">
      <el-table-column prop="name" label="事件名称" min-width="150" />
      <el-table-column prop="team" label="团队" width="120" />
      <el-table-column prop="monitor" label="监控" width="100" />
      <el-table-column prop="rule" label="告警规则" min-width="200" />
      <el-table-column prop="tags" label="标签" min-width="100">
        <template #default="scope">
          <el-tag v-for="tag in scope.row.tags" :key="tag" size="small" style="margin-right: 5px;">{{ tag }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="level" label="策略等级" width="100" />
      <el-table-column prop="alertCount" label="告警数量" width="100">
        <template #default="scope">
          <el-button @click="viewAlertEvents(scope.row)">{{ scope.row.alertCount }}</el-button>
        </template>
      </el-table-column>
      <el-table-column prop="createTime" label="创建时间" width="180" sortable />
      <el-table-column prop="status" label="启用/禁用" width="100">
        <template #default="scope">
          <el-switch v-model="scope.row.status" @change="toggleStatus(scope.row)" />
        </template>
      </el-table-column>
      <el-table-column prop="endpoint" label="推送端点" width="100" />
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="scope">
          <el-button size="small" @click="editAlertStrategy(scope.row)">
            <el-icon><Edit /></el-icon>
          </el-button>
          <el-button size="small" type="danger" @click="deleteAlertStrategy(scope.row)">
            <el-icon><Delete /></el-icon>
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination mt-4">
      <div class="pagination-info">
        共 {{ alertStrategyTotal }} 条
      </div>
      <el-pagination
        background
        layout="prev, pager, next, jumper"
        :total="alertStrategyTotal"
        :page-size="pagination.pageSize"
        :current-page="pagination.currentPage"
        @current-change="handleCurrentChange"
      />
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Edit, Delete } from '@element-plus/icons-vue'

// 定义事件
const emit = defineEmits<{
  (e: 'view-events', strategy: any): void
}>()

// 搜索表单
const searchForm = ref({
  keyword: ''
})

// 分页
const pagination = reactive({
  pageSize: 10,
  currentPage: 1
})

// 告警策略数据
const alertStrategies = ref([
  {
    id: '1',
    name: 'test33333333',
    team: 'Default团队',
    monitor: '指标',
    rule: '过滤器 N/A | 分组: * | 致命: Avg...',
    tags: [],
    level: '高',
    alertCount: 1397,
    createTime: '2024-05-17 15:52:54',
    status: true,
    endpoint: '--'
  },
  {
    id: '2',
    name: '【告警】系统告警',
    team: 'Default团队',
    monitor: '指标',
    rule: '过滤器 policy.app_type = 系统, event_level = 告警',
    tags: ['liqian'],
    level: '高',
    alertCount: 0,
    createTime: '2024-04-18 16:07:23',
    status: true,
    endpoint: 'Email1'
  },
  {
    id: '3',
    name: '【警告】系统警告',
    team: 'Default团队',
    monitor: '指标',
    rule: '过滤器 policy.app_type = 系统, event_level = 警告',
    tags: ['liqian'],
    level: '中',
    alertCount: 1361,
    createTime: '2024-04-18 16:07:11',
    status: true,
    endpoint: 'Email1'
  },
  {
    id: '4',
    name: '【致命】系统告警',
    team: 'Default团队',
    monitor: '指标',
    rule: '过滤器 policy.app_type = 系统, event_level = 致命',
    tags: ['liqian'],
    level: '高',
    alertCount: 218,
    createTime: '2024-04-18 16:07:06',
    status: true,
    endpoint: 'Email1'
  },
  {
    id: '5',
    name: 'SandBox_请求检测告警',
    team: 'Default团队',
    monitor: '指标',
    rule: '过滤器 pod.cluster = SandBox...',
    tags: [],
    level: '高',
    alertCount: 171,
    createTime: '2024-04-18 14:40:50',
    status: true,
    endpoint: 'HTTP1'
  },
  {
    id: '6',
    name: '数据库节点连接异常 (ingester.chir...',
    team: 'Default团队',
    monitor: '系统',
    rule: '过滤器 N/A | 分组: tag.host: ...',
    tags: [],
    level: '高',
    alertCount: 0,
    createTime: '2024-04-11 07:26:38',
    status: true,
    endpoint: '--'
  },
  {
    id: '7',
    name: '数据库节点连接异常 (ingester.queue...',
    team: 'Default团队',
    monitor: '系统',
    rule: '过滤器 N/A | 分组: tag.host: ...',
    tags: [],
    level: '高',
    alertCount: 0,
    createTime: '2024-04-11 07:26:38',
    status: true,
    endpoint: '--'
  },
  {
    id: '8',
    name: '数据库节点连接异常 (ingester.recv...',
    team: 'Default团队',
    monitor: '系统',
    rule: '过滤器 N/A | 分组: tag.host: ...',
    tags: [],
    level: '高',
    alertCount: 2869,
    createTime: '2024-04-11 07:26:38',
    status: true,
    endpoint: '--'
  },
  {
    id: '9',
    name: '事件通知发送方式 (collect.sender.me...',
    team: 'Default团队',
    monitor: '系统',
    rule: '过滤器 N/A | 分组: tag.host: ...',
    tags: [],
    level: '高',
    alertCount: 15,
    createTime: '2024-04-11 07:26:38',
    status: true,
    endpoint: '--'
  }
])

const alertStrategyTotal = ref(55)

// 搜索处理
const handleSearch = () => {
  ElMessage.info('功能开发中...')
}

// 分页处理
const handleCurrentChange = (page: number) => {
  pagination.currentPage = page
}

// 新建告警策略
const createAlertStrategy = () => {
  ElMessage.info('功能开发中...')
}

// 查看告警事件
const viewAlertEvents = (strategy: any) => {
  emit('view-events', strategy)
}

// 切换状态
const toggleStatus = (strategy: any) => {
  // 处理状态切换
}

// 编辑告警策略
const editAlertStrategy = (strategy: any) => {
  ElMessage.info('功能开发中...')
}

// 删除告警策略
const deleteAlertStrategy = (strategy: any) => {
  // 处理删除
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.search-filter {
  margin-bottom: 20px;
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

.header-actions {
  display: flex;
  gap: 10px;
}
</style>
