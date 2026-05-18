<template>
  <!-- 事件详情弹窗 -->
  <el-dialog
    :model-value="eventDetailVisible"
    @update:model-value="$emit('update:eventDetailVisible', $event)"
    title="告警事件详情"
    width="800px"
  >
    <div class="event-detail-content">
      <div class="detail-section">
        <h3>基本信息</h3>
        <el-descriptions :column="2">
          <el-descriptions-item label="事件ID">{{ event?.id }}</el-descriptions-item>
          <el-descriptions-item label="事件等级">{{ event?.level }}</el-descriptions-item>
          <el-descriptions-item label="事件名称">{{ event?.name }}</el-descriptions-item>
          <el-descriptions-item label="监控对象">{{ event?.monitorObject }}</el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ event?.startTime }}</el-descriptions-item>
          <el-descriptions-item label="结束时间">{{ event?.endTime }}</el-descriptions-item>
          <el-descriptions-item label="创建标签">{{ event?.creator }}</el-descriptions-item>
          <el-descriptions-item label="区域">{{ event?.region }}</el-descriptions-item>
        </el-descriptions>
      </div>
      <div class="detail-section">
        <h3>事件信息</h3>
        <el-descriptions :column="1">
          <el-descriptions-item label="事件响应时间">{{ event?.eventInfo }}</el-descriptions-item>
          <el-descriptions-item label="触发条件">{{ event?.triggerCondition }}</el-descriptions-item>
          <el-descriptions-item label="处理建议">{{ event?.suggestion }}</el-descriptions-item>
        </el-descriptions>
      </div>
    </div>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="$emit('update:eventDetailVisible', false)">关闭</el-button>
      </span>
    </template>
  </el-dialog>

  <!-- 列选择弹窗 -->
  <el-dialog
    :model-value="columnSelectVisible"
    @update:model-value="$emit('update:columnSelectVisible', $event)"
    title="列选择"
    width="500px"
  >
    <el-checkbox-group :model-value="selectedColumns" @update:model-value="handleColumnChange">
      <el-checkbox label="开始时间">开始时间</el-checkbox>
      <el-checkbox label="事件名称">事件名称</el-checkbox>
      <el-checkbox label="事件等级">事件等级</el-checkbox>
      <el-checkbox label="监控对象">监控对象</el-checkbox>
      <el-checkbox label="事件信息">事件信息</el-checkbox>
      <el-checkbox label="创建标签">创建标签</el-checkbox>
      <el-checkbox label="区域">区域</el-checkbox>
      <el-checkbox label="事件UID">事件UID</el-checkbox>
    </el-checkbox-group>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="$emit('update:columnSelectVisible', false)">取消</el-button>
        <el-button type="primary" @click="handleConfirmColumns">确定</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
// 定义 Props
const props = defineProps<{
  eventDetailVisible: boolean
  columnSelectVisible: boolean
  event: any
  selectedColumns: string[]
}>()

// 定义事件
const emit = defineEmits<{
  (e: 'update:eventDetailVisible', value: boolean): void
  (e: 'update:columnSelectVisible', value: boolean): void
  (e: 'confirm-columns', columns: string[]): void
}>()

// 本地列选择
const localSelectedColumns = defineModel<string[]>('selectedColumns', { default: () => [] })

// 处理列选择变化
const handleColumnChange = (value: string[]) => {
  localSelectedColumns.value = value
}

// 确认列选择
const handleConfirmColumns = () => {
  emit('confirm-columns', localSelectedColumns.value)
  emit('update:columnSelectVisible', false)
}
</script>

<style scoped>
.event-detail-content {
  padding: 10px;
}

.detail-section {
  margin-bottom: 20px;
}

.detail-section h3 {
  margin-top: 0;
  margin-bottom: 15px;
  font-size: 16px;
  font-weight: bold;
  border-bottom: 1px solid #ebeef5;
  padding-bottom: 10px;
}
</style>
