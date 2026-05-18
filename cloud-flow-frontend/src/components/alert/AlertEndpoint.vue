<template>
  <el-card class="mb-4">
    <template #header>
      <div class="card-header">
        <h2>推送端点</h2>
      </div>
    </template>

    <!-- 标签页 -->
    <el-tabs v-model="activeTab">
      <!-- Email 推送标签页 -->
      <el-tab-pane label="Email推送" name="email">
        <div class="action-buttons mb-4">
          <el-button type="primary" @click="handleCreate('email')">新建连接Email推送</el-button>
        </div>

        <div class="search-filter mb-4">
          <el-form :inline="true" :model="searchForm" class="demo-form-inline">
            <el-form-item label="Email推送列表（最多8项）">
              <el-input v-model="searchForm.keyword" placeholder="搜索关键词" style="width: 300px;" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="handleSearch">搜索</el-button>
            </el-form-item>
          </el-form>
        </div>

        <el-table :data="emailEndpoints" style="width: 100%">
          <el-table-column prop="name" label="名称" min-width="150" sortable />
          <el-table-column prop="team" label="团队" width="120" />
          <el-table-column prop="email" label="推送邮箱" min-width="200" />
          <el-table-column prop="alertType" label="通知类型" width="150" />
          <el-table-column prop="alertCount" label="关联告警策略" width="120">
            <template #default="scope">
              <el-button @click="viewAlertStrategies(scope.row)">{{ scope.row.alertCount }}</el-button>
            </template>
          </el-table-column>
          <el-table-column prop="sourceAccount" label="来源账号" width="150" />
          <el-table-column prop="updateTime" label="更新时间" width="180" sortable />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="scope">
              <el-button size="small" @click="handleEdit(scope.row, 'email')">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button size="small" type="danger" @click="handleDelete(scope.row, 'email')">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination mt-4">
          <div class="pagination-info">共 {{ emailEndpointTotal }} 条</div>
          <el-pagination
            background
            layout="prev, pager, next, jumper"
            :total="emailEndpointTotal"
            :page-size="pagination.pageSize"
            :current-page="pagination.currentPage"
            @current-change="handleCurrentChange"
          />
        </div>
      </el-tab-pane>

      <!-- HTTP 推送标签页 -->
      <el-tab-pane label="HTTP推送" name="http">
        <div class="action-buttons mb-4">
          <el-button type="primary" @click="handleCreate('http')">新建连接HTTP推送</el-button>
        </div>

        <div class="search-filter mb-4">
          <el-form :inline="true" :model="searchForm" class="demo-form-inline">
            <el-form-item label="HTTP推送列表（最多10项）">
              <el-input v-model="searchForm.keyword" placeholder="搜索关键词" style="width: 300px;" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="handleSearch">搜索</el-button>
            </el-form-item>
          </el-form>
        </div>

        <el-table :data="httpEndpoints" style="width: 100%">
          <el-table-column prop="name" label="名称" min-width="150" sortable />
          <el-table-column prop="team" label="团队" width="120" />
          <el-table-column prop="method" label="推送方式" width="120" />
          <el-table-column prop="header" label="Header" width="150" />
          <el-table-column prop="url" label="推送URL" min-width="200" />
          <el-table-column prop="alertType" label="通知类型" width="150" />
          <el-table-column prop="alertCount" label="关联告警策略" width="120">
            <template #default="scope">
              <el-button @click="viewAlertStrategies(scope.row)">{{ scope.row.alertCount }}</el-button>
            </template>
          </el-table-column>
          <el-table-column prop="sourceAccount" label="来源账号" width="150" />
          <el-table-column prop="updateTime" label="更新时间" width="180" sortable />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="scope">
              <el-button size="small" @click="handleEdit(scope.row, 'http')">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button size="small" type="danger" @click="handleDelete(scope.row, 'http')">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination mt-4">
          <div class="pagination-info">共 {{ httpEndpointTotal }} 条</div>
          <el-pagination
            background
            layout="prev, pager, next, jumper"
            :total="httpEndpointTotal"
            :page-size="pagination.pageSize"
            :current-page="pagination.currentPage"
            @current-change="handleCurrentChange"
          />
        </div>
      </el-tab-pane>

      <!-- Kafka 推送标签页 -->
      <el-tab-pane label="Kafka推送" name="kafka">
        <div class="action-buttons mb-4">
          <el-button type="primary" @click="handleCreate('kafka')">新建连接Kafka推送</el-button>
        </div>

        <div class="search-filter mb-4">
          <el-form :inline="true" :model="searchForm" class="demo-form-inline">
            <el-form-item label="Kafka推送列表（最多10项）">
              <el-input v-model="searchForm.keyword" placeholder="搜索关键词" style="width: 300px;" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="handleSearch">搜索</el-button>
            </el-form-item>
          </el-form>
        </div>

        <el-table :data="kafkaEndpoints" style="width: 100%">
          <el-table-column prop="name" label="名称" min-width="150" sortable />
          <el-table-column prop="team" label="团队" width="120" />
          <el-table-column prop="broker" label="Broker" min-width="200" />
          <el-table-column prop="topic" label="Topic" width="150" />
          <el-table-column prop="sasl" label="SASL" width="120" />
          <el-table-column prop="alertCount" label="关联告警策略" width="120">
            <template #default="scope">
              <el-button @click="viewAlertStrategies(scope.row)">{{ scope.row.alertCount }}</el-button>
            </template>
          </el-table-column>
          <el-table-column prop="sourceAccount" label="来源账号" width="150" />
          <el-table-column prop="updateTime" label="更新时间" width="180" sortable />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="scope">
              <el-button size="small" @click="handleEdit(scope.row, 'kafka')">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button size="small" type="danger" @click="handleDelete(scope.row, 'kafka')">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination mt-4">
          <div class="pagination-info">共 {{ kafkaEndpointTotal }} 条</div>
          <el-pagination
            background
            layout="prev, pager, next, jumper"
            :total="kafkaEndpointTotal"
            :page-size="pagination.pageSize"
            :current-page="pagination.currentPage"
            @current-change="handleCurrentChange"
          />
        </div>
      </el-tab-pane>

      <!-- PCAP 策略标签页 -->
      <el-tab-pane label="PCAP策略" name="pcap">
        <div class="action-buttons mb-4">
          <el-button type="primary" @click="handleCreate('pcap')">新建连接PCAP策略</el-button>
        </div>

        <div class="search-filter mb-4">
          <el-form :inline="true" :model="searchForm" class="demo-form-inline">
            <el-form-item label="PCAP策略列表（共1项）">
              <el-input v-model="searchForm.keyword" placeholder="搜索关键词" style="width: 300px;" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="handleSearch">搜索</el-button>
            </el-form-item>
          </el-form>
        </div>

        <el-table :data="pcapEndpoints" style="width: 100%">
          <el-table-column prop="name" label="名称" min-width="150" sortable />
          <el-table-column prop="team" label="团队" width="120" />
          <el-table-column prop="pcapPolicy" label="关联PCAP策略" min-width="200" />
          <el-table-column prop="alertCount" label="关联告警策略" width="120">
            <template #default="scope">
              <el-button @click="viewAlertStrategies(scope.row)">{{ scope.row.alertCount }}</el-button>
            </template>
          </el-table-column>
          <el-table-column prop="sourceAccount" label="来源账号" width="150" />
          <el-table-column prop="updateTime" label="更新时间" width="180" sortable />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="scope">
              <el-button size="small" @click="handleEdit(scope.row, 'pcap')">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button size="small" type="danger" @click="handleDelete(scope.row, 'pcap')">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination mt-4">
          <div class="pagination-info">共 {{ pcapEndpointTotal }} 条</div>
          <el-pagination
            background
            layout="prev, pager, next, jumper"
            :total="pcapEndpointTotal"
            :page-size="pagination.pageSize"
            :current-page="pagination.currentPage"
            @current-change="handleCurrentChange"
          />
        </div>
      </el-tab-pane>

      <!-- Syslog 推送标签页 -->
      <el-tab-pane label="Syslog推送" name="syslog">
        <div class="action-buttons mb-4">
          <el-button type="primary" @click="handleCreate('syslog')">新建连接Syslog推送</el-button>
        </div>

        <div class="search-filter mb-4">
          <el-form :inline="true" :model="searchForm" class="demo-form-inline">
            <el-form-item label="Syslog推送列表（最多10项）">
              <el-input v-model="searchForm.keyword" placeholder="搜索关键词" style="width: 300px;" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="handleSearch">搜索</el-button>
            </el-form-item>
          </el-form>
        </div>

        <el-table :data="syslogEndpoints" style="width: 100%">
          <el-table-column prop="name" label="名称" min-width="150" sortable />
          <el-table-column prop="team" label="团队" width="120" />
          <el-table-column prop="destination" label="推送目的端" min-width="200" />
          <el-table-column prop="alertType" label="通知类型" width="150" />
          <el-table-column prop="alertCount" label="关联告警策略" width="120">
            <template #default="scope">
              <el-button @click="viewAlertStrategies(scope.row)">{{ scope.row.alertCount }}</el-button>
            </template>
          </el-table-column>
          <el-table-column prop="sourceAccount" label="来源账号" width="150" />
          <el-table-column prop="updateTime" label="更新时间" width="180" sortable />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="scope">
              <el-button size="small" @click="handleEdit(scope.row, 'syslog')">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button size="small" type="danger" @click="handleDelete(scope.row, 'syslog')">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination mt-4">
          <div class="pagination-info">共 {{ syslogEndpointTotal }} 条</div>
          <el-pagination
            background
            layout="prev, pager, next, jumper"
            :total="syslogEndpointTotal"
            :page-size="pagination.pageSize"
            :current-page="pagination.currentPage"
            @current-change="handleCurrentChange"
          />
        </div>
      </el-tab-pane>
    </el-tabs>
  </el-card>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Edit, Delete } from '@element-plus/icons-vue'

// 定义事件
const emit = defineEmits<{
  (e: 'create', type: string): void
  (e: 'edit', endpoint: any, type: string): void
  (e: 'delete', endpoint: any, type: string): void
}>()

// 当前标签页
const activeTab = ref('email')

// 搜索表单
const searchForm = ref({
  keyword: ''
})

// 分页
const pagination = reactive({
  pageSize: 10,
  currentPage: 1
})

// Email 推送数据
const emailEndpoints = ref([
  { id: '1', name: '测试', team: '--', email: 'zhui@example.com', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'zhui@example.com', updateTime: '2023-09-04 21:30:14' },
  { id: '2', name: '测试', team: '--', email: 'chyi@example.com', alertType: '告警开启', alertCount: 1, sourceAccount: 'chyi@example.com', updateTime: '2023-02-21 10:17:50' },
  { id: '3', name: 'zhan', team: '--', email: 'zhan@example.com', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'ye@example.com', updateTime: '2023-01-04 17:19:51' },
  { id: '4', name: 'min', team: '--', email: 'min@example.com', alertType: '告警开启 告警聚合', alertCount: 1, sourceAccount: '17@example.com', updateTime: '2022-12-07 15:37:09' },
  { id: '5', name: '1', team: '--', email: 'chong@example.com', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'chong@example.com', updateTime: '2022-07-04 10:46:33' },
  { id: '6', name: 'test', team: '--', email: 'teq@example.com', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'xc@example.com', updateTime: '2022-06-09 10:44:43' },
  { id: '7', name: 'liug', team: '--', email: 'liug@example.com', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'xc@example.com', updateTime: '2022-06-09 10:43:40' },
  { id: '8', name: '1', team: '--', email: 'wei@example.com', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'wei@example.com', updateTime: '2022-05-24 14:33:09' },
  { id: '9', name: '测试', team: '--', email: 'he@example.com', alertType: '告警开启 告警聚合', alertCount: 1, sourceAccount: 'xc@example.com', updateTime: '2022-03-15 16:36:12' },
  { id: '10', name: 'Meil', team: '--', email: 'chyi@example.com', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'chyi@example.com', updateTime: '2022-03-02 16:20:13' }
])
const emailEndpointTotal = ref(18)

// HTTP 推送数据
const httpEndpoints = ref([
  { id: '1', name: '云原生电商应用2', team: '--', method: 'POST', header: 'Content-Type:application/json', url: 'https://example.com', alertType: '告警开启 告警聚合', alertCount: 1, sourceAccount: 'xc@example.com', updateTime: '2023-09-28 09:59:16' },
  { id: '2', name: '飞书推送', team: '--', method: 'POST', header: 'Content-Type:application/json', url: 'https://example.com', alertType: '告警开启 告警聚合', alertCount: 2, sourceAccount: 'yile@example.com', updateTime: '2022-01-06 15:35:08' },
  { id: '3', name: 'qa-survey', team: '内部测试', method: 'POST', header: '--', url: 'http://11', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'pm@example.com', updateTime: '2021-11-02 16:20:19' }
])
const httpEndpointTotal = ref(3)

// Kafka 推送数据
const kafkaEndpoints = ref([
  { id: '1', name: '测试', team: 'Default团队', broker: '192.168.1.1:9092, 192.168.1.2:9092', topic: 'alert-topic', sasl: 'Plain', alertCount: 0, sourceAccount: 'test@example.com', updateTime: '2023-01-01 00:00:00' }
])
const kafkaEndpointTotal = ref(6)

// PCAP 策略数据
const pcapEndpoints = ref([
  { id: '1', name: '测试PCAP策略', team: 'Default团队', pcapPolicy: 'PCAP策略1', alertCount: 0, sourceAccount: 'test@example.com', updateTime: '2023-01-01 00:00:00' }
])
const pcapEndpointTotal = ref(5)

// Syslog 推送数据
const syslogEndpoints = ref([
  { id: '1', name: '测试Syslog推送', team: 'Default团队', destination: 'udp://192.168.1.1:514', alertType: '告警开启 告警聚合', alertCount: 0, sourceAccount: 'test@example.com', updateTime: '2023-01-01 00:00:00' }
])
const syslogEndpointTotal = ref(4)

// 搜索处理
const handleSearch = () => {
  ElMessage.info('功能开发中...')
}

// 分页处理
const handleCurrentChange = (page: number) => {
  pagination.currentPage = page
}

// 查看告警策略
const viewAlertStrategies = (endpoint: any) => {
  // 处理查看
}

// 创建端点
const handleCreate = (type: string) => {
  emit('create', type)
}

// 编辑端点
const handleEdit = (endpoint: any, type: string) => {
  emit('edit', endpoint, type)
}

// 删除端点
const handleDelete = (endpoint: any, type: string) => {
  emit('delete', endpoint, type)
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

.action-buttons {
  margin-bottom: 20px;
}
</style>
