<template>
  <div class="alert">
    <!-- 告警策略 -->
    <AlertStrategy
      v-if="currentModule === 'strategy'"
      @view-events="handleViewEvents"
    />

    <!-- 推送端点 -->
    <AlertEndpoint
      v-if="currentModule === 'endpoint'"
      @create="handleCreateEndpoint"
      @edit="handleEditEndpoint"
      @delete="handleDeleteEndpoint"
    />

    <!-- 告警事件 -->
    <AlertEvent
      v-if="currentModule === 'event'"
      @view-detail="handleViewEventDetail"
      @export="handleExportEvents"
      @toggle-columns="handleToggleColumns"
    />

    <!-- 端点表单弹窗 -->
    <AlertEndpointForm
      :visible="endpointFormVisible"
      :type="endpointFormType"
      @confirm="handleConfirmEndpointForm"
      @cancel="handleCancelEndpointForm"
      @update:visible="endpointFormVisible = $event"
    />

    <!-- 事件详情和列选择弹窗 -->
    <AlertEventDialog
      :event-detail-visible="eventDetailVisible"
      :column-select-visible="columnSelectVisible"
      :event="selectedEvent"
      :selected-columns="selectedColumns"
      @update:event-detail-visible="eventDetailVisible = $event"
      @update:column-select-visible="columnSelectVisible = $event"
      @confirm-columns="handleConfirmColumns"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'

import AlertStrategy from '../components/alert/AlertStrategy.vue'
import AlertEndpoint from '../components/alert/AlertEndpoint.vue'
import AlertEndpointForm from '../components/alert/AlertEndpointForm.vue'
import AlertEvent from '../components/alert/AlertEvent.vue'
import AlertEventDialog from '../components/alert/AlertEventDialog.vue'

// 路由
const route = useRoute()

// 当前模块
const currentModule = computed(() => {
  return route.path.split('/').pop() || 'strategy'
})

// 端点表单状态
const endpointFormVisible = ref(false)
const endpointFormType = ref('email')

// 事件详情弹窗状态
const eventDetailVisible = ref(false)
const columnSelectVisible = ref(false)

// 选中的事件
const selectedEvent = ref(null)

// 选中的列
const selectedColumns = ref(['事件名称', '事件级别', '事件状态', '触发条件', '通知方式', '存储分析'])

// 查看告警事件（从策略页面跳转）
const handleViewEvents = (strategy: any) => {
  ElMessage.info('功能开发中...')
}

// 创建端点
const handleCreateEndpoint = (type: string) => {
  endpointFormType.value = type
  endpointFormVisible.value = true
}

// 编辑端点
const handleEditEndpoint = (endpoint: any, type: string) => {
  endpointFormType.value = type
  endpointFormVisible.value = true
  ElMessage.info('功能开发中...')
}

// 删除端点
const handleDeleteEndpoint = (endpoint: any, type: string) => {
  ElMessage.info('功能开发中...')
}

// 确认端点表单
const handleConfirmEndpointForm = (form: any) => {
  ElMessage.success('保存成功')
}

// 取消端点表单
const handleCancelEndpointForm = () => {
  endpointFormVisible.value = false
}

// 查看事件详情
const handleViewEventDetail = (event: any) => {
  selectedEvent.value = event
  eventDetailVisible.value = true
}

// 导出事件
const handleExportEvents = () => {
  ElMessage.info('功能开发中...')
}

// 列选择
const handleToggleColumns = () => {
  columnSelectVisible.value = true
}

// 确认列选择
const handleConfirmColumns = (columns: string[]) => {
  selectedColumns.value = columns
  ElMessage.success('列选择已更新')
}
</script>

<style scoped>
.alert {
  padding: 20px;
}
</style>
