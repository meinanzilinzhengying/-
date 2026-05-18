<template>
  <div class="metrics-summary-content">
    <!-- 顶部控制 -->
    <div class="summary-header">
      <div class="metrics-collection">
        <el-form :inline="true" :model="summaryForm" class="demo-form-inline">
          <el-form-item label="指标集合">
            <el-select v-model="summaryForm.collection" placeholder="选择指标集合" style="width: 150px;">
              <el-option label="网络" value="network" />
              <el-option label="系统" value="system" />
              <el-option label="应用" value="application" />
              <el-option label="数据库" value="database" />
            </el-select>
          </el-form-item>
          <el-form-item label="数据库">
            <el-select v-model="summaryForm.table" placeholder="选择数据库" style="width: 150px;">
              <el-option label="数据库表A" value="tableA" />
              <el-option label="数据库表B" value="tableB" />
              <el-option label="数据库表C" value="tableC" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="refreshSummary">
              <el-icon><Refresh /></el-icon> 重新检索
            </el-button>
          </el-form-item>
        </el-form>
      </div>
    </div>

    <!-- 标签页切换 -->
    <div class="summary-tabs">
      <el-tabs v-model="summaryActiveTab">
        <el-tab-pane label="METRICS" name="metrics">
          <!-- Metrics 表格 -->
          <div class="metrics-table">
            <el-table :data="metricsData" style="width: 100%">
              <el-table-column prop="name" label="name" width="120" />
              <el-table-column prop="unit" label="unit" width="80" />
              <el-table-column prop="type" label="Type" width="100" />
              <el-table-column prop="displayName" label="display_name" width="120" />
              <el-table-column prop="category" label="category" width="120" />
              <el-table-column prop="description" label="响应时间" min-width="200" />
            </el-table>

            <!-- 分页 -->
            <div class="pagination mt-4">
              <div class="pagination-info">
                共 {{ metricsTotal }} 条
              </div>
              <el-pagination
                background
                layout="prev, pager, next, jumper"
                :total="metricsTotal"
                :page-size="metricsPageSize"
                :current-page="metricsCurrentPage"
                @current-change="handleMetricsPageChange"
              />
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane label="TAG" name="tag">
          <!-- Tag 表格 -->
          <div class="tag-table">
            <el-table :data="tagData" style="width: 100%">
              <el-table-column prop="name" label="name" width="120" />
              <el-table-column prop="description" label="响应时间" min-width="200" />
              <el-table-column prop="type" label="类型" width="100" />
              <el-table-column prop="example" label="示例" min-width="150" />
            </el-table>

            <!-- 分页 -->
            <div class="pagination mt-4">
              <div class="pagination-info">
                共 {{ tagTotal }} 条
              </div>
              <el-pagination
                background
                layout="prev, pager, next, jumper"
                :total="tagTotal"
                :page-size="tagPageSize"
                :current-page="tagCurrentPage"
                @current-change="handleTagPageChange"
              />
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'

// 摘要表单
const summaryForm = ref({
  collection: 'network',
  table: 'tableA'
})

// 摘要标签页
const summaryActiveTab = ref('metrics')

// Metrics 数据
const metricsData = ref([
  {
    name: 'name',
    unit: '字节',
    type: 'counter',
    displayName: '字节',
    category: 'L3 Throughput',
    description: '发送的字节数总和，包含 Ethernet 头的所有内容',
  },
  {
    name: 'byte_tx',
    unit: '字节',
    type: 'counter',
    displayName: '发送字节',
    category: 'L3 Throughput',
    description: '发送的字节数总和，包含 Ethernet 头的所有内容',
  },
  {
    name: 'byte_rx',
    unit: '字节',
    type: 'counter',
    displayName: '接收字节',
    category: 'L3 Throughput',
    description: '接收的字节数总和，包含 Ethernet 头的所有内容',
  },
  {
    name: 'packet_tx',
    unit: '个',
    type: 'counter',
    displayName: '发送包',
    category: 'L3 Throughput',
    description: '发送的总数据包数',
  },
  {
    name: 'packet_rx',
    unit: '个',
    type: 'counter',
    displayName: '接收包数',
    category: 'L3 Throughput',
    description: '接收的总数据库包',
  },
  {
    name: 'l3_byte_tx',
    unit: '字节',
    type: 'counter',
    displayName: '发送网络层字节',
    category: 'L3 Throughput',
    description: '发送的字节数总和，包含IP 头但无网络层字节',
  },
  {
    name: 'l3_byte_rx',
    unit: '字节',
    type: 'counter',
    displayName: '接收字节数总和',
    category: 'L3 Throughput',
    description: '接收的字节数总和，包含IP 头但无网络层字节',
  },
  {
    name: 'bps_tx',
    unit: '字节',
    type: 'gauge',
    displayName: '平均发送速率',
    category: 'L3 Throughput',
    description: '平均发送速率，单位：字节/秒（统计周期内），等于tx_byte / 统计周期（秒）',
  },
  {
    name: 'bps_rx',
    unit: '字节',
    type: 'gauge',
    displayName: '平均接收速率',
    category: 'L3 Throughput',
    description: '平均接收速率，单位：字节/秒（统计周期内），等于rx_byte / 统计周期（秒）',
  },
  {
    name: 'new_flow',
    unit: '个',
    type: 'counter',
    displayName: '新增流组',
    category: 'L4 Throughput',
    description: '在这个统计周期内的新增服务数量，重建流不计入',
  }
])

// Metrics 分页相关变量
const metricsPageSize = ref(10)
const metricsCurrentPage = ref(1)
const metricsTotal = ref(99)

// Tag 数据
const tagData = ref([
  {
    name: 'src_ip',
    description: '源IP 地址',
    type: 'string',
    example: '192.168.1.1'
  },
  {
    name: 'dst_ip',
    description: '目标 IP 地址',
    type: 'string',
    example: '192.168.1.2'
  },
  {
    name: 'src_port',
    description: '源端口',
    type: 'integer',
    example: '8080'
  },
  {
    name: 'dst_port',
    description: '目标端口',
    type: 'integer',
    example: '80'
  },
  {
    name: 'protocol',
    description: '协议类型',
    type: 'string',
    example: 'TCP'
  },
  {
    name: 'application',
    description: '应用名称',
    type: 'string',
    example: 'nginx'
  },
  {
    name: 'host',
    description: '主机名称',
    type: 'string',
    example: 'server-01'
  },
  {
    name: 'region',
    description: '区域',
    type: 'string',
    example: 'cn-beijing'
  },
  {
    name: 'environment',
    description: '环境',
    type: 'string',
    example: 'production'
  },
  {
    name: 'service',
    description: '服务名称',
    type: 'string',
    example: 'api-gateway'
  }
])

// Tag 分页相关变量
const tagPageSize = ref(10)
const tagCurrentPage = ref(1)
const tagTotal = ref(50)

// 刷新摘要数据
const refreshSummary = () => {
  // 使用模拟数据
}

// Metrics 分页变化
const handleMetricsPageChange = (page: number) => {
  metricsCurrentPage.value = page
}

// Tag 分页变化
const handleTagPageChange = (page: number) => {
  tagCurrentPage.value = page
}
</script>

<style scoped>
.metrics-summary-content {
  padding: 20px;
}

.summary-header {
  margin-bottom: 20px;
  padding: 15px;
  background-color: #f5f7fa;
  border-radius: 4px;
}

.metrics-collection {
  display: flex;
  align-items: center;
}

.summary-tabs {
  margin-top: 20px;
}

.metrics-table,
.tag-table {
  background-color: white;
  border-radius: 4px;
  padding: 15px;
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
</style>
