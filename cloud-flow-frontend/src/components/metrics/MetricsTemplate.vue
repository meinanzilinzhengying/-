<template>
  <div class="metrics-template-content">
    <!-- 顶部控制 -->
    <div class="template-header">
      <el-button type="primary" @click="openCreateTemplateDialog">
        <el-icon><Plus /></el-icon> 新建模板
      </el-button>
      <div class="template-filters">
        <el-form :inline="true" :model="templateForm" class="demo-form-inline">
          <el-form-item label="数据库">
            <el-select v-model="templateForm.table" placeholder="选择数据库" style="width: 150px;">
              <el-option label="全部" value="all" />
              <el-option label="网络" value="network" />
              <el-option label="系统" value="system" />
              <el-option label="应用" value="application" />
              <el-option label="数据库" value="database" />
            </el-select>
          </el-form-item>
          <el-form-item label="网络指标">
            <el-select v-model="templateForm.service" placeholder="选择服务指标" style="width: 150px;">
              <el-option label="全部" value="all" />
              <el-option label="流量监控" value="traffic" />
              <el-option label="性能监控" value="performance" />
              <el-option label="错误监控" value="error" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-input v-model="templateForm.search" placeholder="搜索关键词" style="width: 200px;" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="searchTemplates">
              <el-icon><Search /></el-icon> 搜索
            </el-button>
          </el-form-item>
        </el-form>
      </div>
    </div>

    <!-- 模板列表 -->
    <div class="template-list">
      <el-table :data="templates" style="width: 100%">
        <el-table-column type="expand">
          <template #default="scope">
            <el-table :data="scope.row.metrics" style="width: 100%">
              <el-table-column prop="name" label="指标策略" width="120" />
              <el-table-column prop="alias" label="别名" width="120" />
              <el-table-column prop="aggregation" label="聚合" width="80" />
              <el-table-column prop="function" label="函数" width="100" />
              <el-table-column prop="unit" label="单位" width="80" />
              <el-table-column prop="threshold" label="阈值" width="80" />
            </el-table>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" width="120" />
        <el-table-column prop="team" label="团队" width="120" />
        <el-table-column prop="database" label="数据库" width="120" />
        <el-table-column prop="metricsType" label="数据库类型" width="120" />
        <el-table-column prop="creator" label="创建人" width="100" />
        <el-table-column prop="lastUsed" label="最近使用时间" width="150" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="scope">
            <el-button
              size="small"
              type="primary"
              @click="editTemplate(scope.row)"
              :disabled="scope.row.isDefault"
            >
              <el-icon><Edit /></el-icon> 编辑
            </el-button>
            <el-button
              size="small"
              type="danger"
              @click="deleteTemplate(scope.row)"
              :disabled="scope.row.isDefault"
            >
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination mt-4">
        <div class="pagination-info">
          共 {{ templateTotal }} 条
        </div>
        <el-pagination
          background
          layout="prev, pager, next, jumper"
          :total="templateTotal"
          :page-size="templatePageSize"
          :current-page="templateCurrentPage"
          @current-change="handleTemplatePageChange"
        />
      </div>
    </div>

    <!-- 新建/编辑模板对话框 -->
    <el-dialog
      v-model="templateDialogVisible"
      :title="isEditTemplate ? '编辑网络指标监控' : '新建网络指标监控'"
      width="800px"
    >
      <el-form :model="newTemplateForm" label-width="100px">
        <el-form-item label="模板策略" required>
          <el-input v-model="newTemplateForm.name" placeholder="请输入模板名" />
        </el-form-item>
        <el-form-item label="团队" required>
          <el-select v-model="newTemplateForm.team" placeholder="选择团队" style="width: 100%;">
            <el-option label="Default团队" value="default" />
            <el-option label="开发团队" value="dev" />
            <el-option label="测试团队" value="test" />
            <el-option label="运维团队" value="ops" />
          </el-select>
        </el-form-item>
        <el-form-item label="数据库" required>
          <el-select v-model="newTemplateForm.database" placeholder="选择数据库" style="width: 100%;">
            <el-option label="应用" value="application" />
            <el-option label="网络" value="network" />
            <el-option label="系统" value="system" />
            <el-option label="数据库" value="database" />
          </el-select>
        </el-form-item>
        <el-form-item label="网络指标" required>
          <el-select v-model="newTemplateForm.service" placeholder="选择服务指标" style="width: 100%;">
            <el-option label="流量监控" value="traffic" />
            <el-option label="性能监控" value="performance" />
            <el-option label="错误监控" value="error" />
          </el-select>
        </el-form-item>

        <!-- 指标列表 -->
        <el-form-item label="指标">
          <div class="metrics-list">
            <div v-for="(metric, index) in newTemplateForm.metrics" :key="index" class="metric-item">
              <el-form :inline="true" :model="metric" class="demo-form-inline">
                <el-form-item>
                  <el-select v-model="metric.name" placeholder="选择指标" style="width: 120px;">
                    <el-option label="请求" value="request" />
                    <el-option label="响应" value="response" />
                    <el-option label="错误" value="error" />
                    <el-option label="平均时延" value="avg_latency" />
                    <el-option label="异常" value="anomaly" />
                  </el-select>
                </el-form-item>
                <el-form-item>
                  <el-select v-model="metric.aggregation" placeholder="聚合" style="width: 80px;">
                    <el-option label="Avg" value="avg" />
                    <el-option label="Sum" value="sum" />
                    <el-option label="Max" value="max" />
                    <el-option label="Min" value="min" />
                  </el-select>
                </el-form-item>
                <el-form-item>
                  <el-select v-model="metric.function" placeholder="函数" style="width: 100px;">
                    <el-option label="PerSecond" value="per_second" />
                    <el-option label="PerMinute" value="per_minute" />
                    <el-option label="Raw" value="raw" />
                  </el-select>
                </el-form-item>
                <el-form-item>
                  <el-input v-model="metric.alias" placeholder="别名" style="width: 120px;" />
                </el-form-item>
                <el-form-item>
                  <el-input v-model="metric.unit" placeholder="单位" style="width: 80px;" />
                </el-form-item>
                <el-form-item>
                  <el-input v-model="metric.threshold" placeholder="阈值" style="width: 80px;" />
                </el-form-item>
                <el-form-item>
                  <el-switch v-model="metric.enabled" style="margin-right: 10px;" />
                </el-form-item>
                <el-form-item>
                  <el-button type="danger" @click="removeTemplateMetric(index)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </el-form-item>
              </el-form>
            </div>

            <!-- 添加指标按钮 -->
            <div class="add-metric">
              <el-button type="primary" @click="addTemplateMetric">
                <el-icon><Plus /></el-icon> 添加指标
              </el-button>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="templateDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="saveTemplate">确定</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Plus, Search, Edit, Delete } from '@element-plus/icons-vue'

// 模板表单
const templateForm = ref({
  table: 'all',
  service: 'all',
  search: ''
})

// 模板模拟数据
const templates = ref([
  {
    id: 1,
    name: '默认模板',
    team: '--',
    database: 'flow_metrics',
    metricsType: 'network',
    creator: 'admin',
    lastUsed: '--',
    isDefault: true,
    metrics: [
      { name: 'byte_tx', alias: '发送速率', aggregation: 'Avg', function: 'PerSecond', unit: '字节/秒', threshold: '--', enabled: true },
      { name: 'byte_rx', alias: '接收速率', aggregation: 'Avg', function: 'PerSecond', unit: '字节/秒', threshold: '--', enabled: true },
      { name: 'packet_tx', alias: '发送吞吐率', aggregation: 'Avg', function: 'PerSecond', unit: '包/秒', threshold: '--', enabled: true },
      { name: 'packet_rx', alias: '接收吞吐率', aggregation: 'Avg', function: 'PerSecond', unit: '包/秒', threshold: '--', enabled: true },
      { name: 'l4_byte_tx', alias: '发送传输层字节', aggregation: 'Avg', function: 'PerSecond', unit: '字节/秒', threshold: '--', enabled: true },
      { name: 'l4_byte_rx', alias: '接收传输层字节', aggregation: 'Avg', function: 'PerSecond', unit: '字节/秒', threshold: '--', enabled: true }
    ]
  },
  {
    id: 2,
    name: '流量盘点',
    team: 'Default团队',
    database: 'flow_metrics',
    metricsType: 'network',
    creator: 'admin',
    lastUsed: '2024-08-16 16:22:49',
    isDefault: false,
    metrics: [
      { name: 'byte_tx', alias: '发送速率', aggregation: 'Avg', function: 'PerSecond', unit: '字节/秒', threshold: '--', enabled: true },
      { name: 'byte_rx', alias: '接收速率', aggregation: 'Avg', function: 'PerSecond', unit: '字节/秒', threshold: '--', enabled: true }
    ]
  },
  {
    id: 3,
    name: '网络延迟异常分析',
    team: 'Default团队',
    database: 'flow_metrics',
    metricsType: 'network',
    creator: 'liqian',
    lastUsed: '2024-08-16 16:22:49',
    isDefault: false,
    metrics: []
  },
  {
    id: 4,
    name: '网络延迟分析',
    team: 'Default团队',
    database: 'flow_metrics',
    metricsType: 'network',
    creator: 'admin',
    lastUsed: '2024-08-16 16:48:46',
    isDefault: false,
    metrics: []
  },
  {
    id: 5,
    name: '网络异常分析',
    team: 'Default团队',
    database: 'flow_metrics',
    metricsType: 'network',
    creator: 'admin',
    lastUsed: '2024-08-16 16:48:16',
    isDefault: false,
    metrics: []
  },
  {
    id: 6,
    name: 'Test',
    team: 'Default团队',
    database: 'flow_metrics',
    metricsType: 'network',
    creator: 'admin',
    lastUsed: '2024-08-16 16:48:16',
    isDefault: false,
    metrics: []
  }
])

// 模板分页
const templatePageSize = ref(10)
const templateCurrentPage = ref(1)
const templateTotal = ref(26)

// 模板对话框
const templateDialogVisible = ref(false)
const isEditTemplate = ref(false)

// 新建模板表单
const newTemplateForm = ref({
  name: '',
  team: 'default',
  database: 'application',
  service: 'traffic',
  metrics: [
    { name: 'request', aggregation: 'avg', function: 'per_second', alias: '', unit: '', threshold: '', enabled: true },
    { name: 'response', aggregation: 'avg', function: 'per_second', alias: '', unit: '', threshold: '', enabled: true },
    { name: 'avg_latency', aggregation: 'avg', function: 'per_second', alias: '', unit: '', threshold: '', enabled: true },
    { name: 'anomaly', aggregation: 'avg', function: 'per_second', alias: '', unit: '', threshold: '10', enabled: true }
  ]
})

// 打开新建模板窗口
const openCreateTemplateDialog = () => {
  isEditTemplate.value = false
  newTemplateForm.value = {
    name: '',
    team: 'default',
    database: 'application',
    service: 'traffic',
    metrics: [
      { name: 'request', aggregation: 'avg', function: 'per_second', alias: '', unit: '', threshold: '', enabled: true }
    ]
  }
  templateDialogVisible.value = true
}

// 搜索模板
const searchTemplates = () => {
  // 模拟搜索
}

// 编辑模板处理函数
const editTemplate = (template: any) => {
  isEditTemplate.value = true
  newTemplateForm.value = {
    name: template.name,
    team: template.team,
    database: template.database,
    service: template.service || 'traffic',
    metrics: template.metrics.map((metric: any) => ({
      name: metric.name,
      aggregation: metric.aggregation,
      function: metric.function,
      alias: metric.alias,
      unit: metric.unit,
      threshold: metric.threshold,
      enabled: metric.enabled
    }))
  }
  templateDialogVisible.value = true
}

// 删除模板处理函数
const deleteTemplate = (template: any) => {
  // 模拟删除
}

// 保存模板
const saveTemplate = () => {
  templateDialogVisible.value = false
  // 模拟保存
}

// 添加模板指标
const addTemplateMetric = () => {
  newTemplateForm.value.metrics.push({
    name: 'request',
    aggregation: 'avg',
    function: 'per_second',
    alias: '',
    unit: '',
    threshold: '',
    enabled: true
  })
}

// 移除模板指标
const removeTemplateMetric = (index: number) => {
  newTemplateForm.value.metrics.splice(index, 1)
}

// 模板分页变化
const handleTemplatePageChange = (page: number) => {
  templateCurrentPage.value = page
}
</script>

<style scoped>
.metrics-template-content {
  padding: 20px;
}

.template-header {
  margin-bottom: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.template-filters {
  display: flex;
  align-items: center;
}

.template-list {
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

.metrics-list {
  width: 100%;
}

.metric-item {
  margin-bottom: 10px;
  padding: 10px;
  background-color: #f9f9f9;
  border-radius: 4px;
}

.add-metric {
  margin-top: 15px;
  text-align: right;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
