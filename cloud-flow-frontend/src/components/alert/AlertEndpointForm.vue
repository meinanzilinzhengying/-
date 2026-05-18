<template>
  <!-- Email 推送表单 -->
  <el-dialog
    :model-value="visible && type === 'email'"
    @update:model-value="handleClose"
    title="新建连接Email推送"
    width="500px"
  >
    <el-form :model="emailForm" label-width="100px">
      <el-form-item label="* 策略">
        <el-input v-model="emailForm.name" placeholder="请输入名称" />
      </el-form-item>
      <el-form-item label="* 团队">
        <el-select v-model="emailForm.team" placeholder="请选择团队">
          <el-option label="Default团队" value="Default团队" />
        </el-select>
      </el-form-item>
      <el-form-item label="响应时间">
        <el-input v-model="emailForm.description" placeholder="可输入55个任意字符" type="textarea" />
      </el-form-item>
      <el-form-item label="* 邮箱">
        <el-input v-model="emailForm.email" placeholder="请输入邮箱" />
      </el-form-item>
      <el-form-item label="推送标题">
        <el-input v-model="emailForm.subject" placeholder="请输入推送标题" />
      </el-form-item>
      <el-form-item label="配置等级">
        <el-select v-model="emailForm.level" placeholder="请选择">
          <el-option label="严重, 错误, 警告, 恢复, 未确认" value="fatal,error,warning,recovery,no_data" />
        </el-select>
      </el-form-item>
      <el-form-item label="* 推送周期">
        <el-select v-model="emailForm.cycle" placeholder="请选择">
          <el-option label="15分钟" value="15min" />
          <el-option label="30分钟" value="30min" />
          <el-option label="1小时" value="1小时" />
        </el-select>
      </el-form-item>
      <el-form-item label="推送频率">
        <div class="frequency-control">
          <el-switch v-model="emailForm.frequencyEnabled" />
          <el-input v-model="emailForm.frequency" style="width: 80px; margin-left: 10px;" />
          <span style="margin-left: 10px;">秒</span>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="testForm">测试</el-button>
        <el-button @click="handleClose(false)">取消</el-button>
        <el-button type="primary" @click="handleConfirm">确定</el-button>
      </span>
    </template>
  </el-dialog>

  <!-- HTTP 推送表单 -->
  <el-dialog
    :model-value="visible && type === 'http'"
    @update:model-value="handleClose"
    title="新建连接HTTP推送"
    width="500px"
  >
    <el-form :model="httpForm" label-width="100px">
      <el-form-item label="* 策略">
        <el-input v-model="httpForm.name" placeholder="请输入名称" />
      </el-form-item>
      <el-form-item label="* 团队">
        <el-select v-model="httpForm.team" placeholder="请选择团队">
          <el-option label="Default团队" value="Default团队" />
        </el-select>
      </el-form-item>
      <el-form-item label="响应时间">
        <el-input v-model="httpForm.description" placeholder="可输入55个任意字符" type="textarea" />
      </el-form-item>
      <el-form-item label="* 推送方式">
        <el-select v-model="httpForm.method" placeholder="请选择">
          <el-option label="POST" value="POST" />
          <el-option label="PUT" value="PUT" />
          <el-option label="PATCH" value="PATCH" />
        </el-select>
      </el-form-item>
      <el-form-item label="* 推送URL">
        <el-input v-model="httpForm.url" placeholder="支持HTTP、HTTPS协议，支持Jinja模板渲染" />
      </el-form-item>
      <el-form-item label="Header">
        <div class="header-control">
          <el-input v-model="httpForm.headerKey" placeholder="key" style="width: 120px;" />
          <span style="margin: 0 10px;">:</span>
          <el-input v-model="httpForm.headerValue" placeholder="value" style="width: 200px;" />
          <el-button type="primary" size="small" style="margin-left: 10px;">+</el-button>
        </div>
      </el-form-item>
      <el-form-item label="推送内容">
        <el-input v-model="httpForm.content" placeholder="支持Jinja模板渲染，默认推送内容参考参数说明" type="textarea" />
        <div class="form-help">
          <el-button @click="showHelp">参数说明</el-button>
          <el-button @click="previewContent">预览</el-button>
        </div>
      </el-form-item>
      <el-form-item label="配置等级">
        <el-select v-model="httpForm.level" placeholder="请选择">
          <el-option label="严重, 错误, 警告, 恢复, 未确认" value="fatal,error,warning,recovery,no_data" />
        </el-select>
      </el-form-item>
      <el-form-item label="* 推送周期">
        <el-select v-model="httpForm.cycle" placeholder="请选择">
          <el-option label="15分钟" value="15min" />
          <el-option label="30分钟" value="30min" />
          <el-option label="1小时" value="1小时" />
        </el-select>
      </el-form-item>
      <el-form-item label="推送频率">
        <div class="frequency-control">
          <el-switch v-model="httpForm.frequencyEnabled" />
          <el-input v-model="httpForm.frequency" style="width: 80px; margin-left: 10px;" />
          <span style="margin-left: 10px;">秒</span>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="testForm">测试</el-button>
        <el-button @click="handleClose(false)">取消</el-button>
        <el-button type="primary" @click="handleConfirm">确定</el-button>
      </span>
    </template>
  </el-dialog>

  <!-- Kafka 推送表单 -->
  <el-dialog
    :model-value="visible && type === 'kafka'"
    @update:model-value="handleClose"
    title="新建连接Kafka推送"
    width="500px"
  >
    <el-form :model="kafkaForm" label-width="100px">
      <el-form-item label="* 策略">
        <el-input v-model="kafkaForm.name" placeholder="请输入名称" />
      </el-form-item>
      <el-form-item label="* 团队">
        <el-select v-model="kafkaForm.team" placeholder="请选择团队">
          <el-option label="Default团队" value="Default团队" />
        </el-select>
      </el-form-item>
      <el-form-item label="响应时间">
        <el-input v-model="kafkaForm.description" placeholder="可输入55个任意字符" type="textarea" />
      </el-form-item>
      <el-form-item label="* Broker地址">
        <el-input v-model="kafkaForm.broker" placeholder="[Broker地址]:[端口],[Broker地址]:[端口]" />
      </el-form-item>
      <el-form-item label="* Topic">
        <el-input v-model="kafkaForm.topic" placeholder="请输入Topic" />
      </el-form-item>
      <el-form-item label="SASL">
        <el-select v-model="kafkaForm.sasl" placeholder="请选择">
          <el-option label="None" value="none" />
          <el-option label="Plain" value="plain" />
        </el-select>
      </el-form-item>
      <el-form-item label="推送内容">
        <el-input v-model="kafkaForm.content" placeholder="支持Jinja模板渲染，默认推送内容参考参数说明" type="textarea" />
        <div class="form-help">
          <el-button @click="showHelp">参数说明</el-button>
          <el-button @click="previewContent">预览</el-button>
        </div>
      </el-form-item>
      <el-form-item label="配置等级">
        <el-select v-model="kafkaForm.level" placeholder="请选择">
          <el-option label="严重, 错误, 警告, 恢复, 未确认" value="fatal,error,warning,recovery,no_data" />
        </el-select>
      </el-form-item>
      <el-form-item label="* 推送周期">
        <el-select v-model="kafkaForm.cycle" placeholder="请选择">
          <el-option label="15分钟" value="15min" />
          <el-option label="30分钟" value="30min" />
          <el-option label="1小时" value="1小时" />
        </el-select>
      </el-form-item>
      <el-form-item label="推送频率">
        <div class="frequency-control">
          <el-switch v-model="kafkaForm.frequencyEnabled" />
          <el-input v-model="kafkaForm.frequency" style="width: 80px; margin-left: 10px;" />
          <span style="margin-left: 10px;">秒</span>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="testForm">测试</el-button>
        <el-button @click="handleClose(false)">取消</el-button>
        <el-button type="primary" @click="handleConfirm">确定</el-button>
      </span>
    </template>
  </el-dialog>

  <!-- PCAP 策略表单 -->
  <el-dialog
    :model-value="visible && type === 'pcap'"
    @update:model-value="handleClose"
    title="新建连接PCAP策略"
    width="500px"
  >
    <el-form :model="pcapForm" label-width="100px">
      <el-form-item label="* 策略">
        <el-input v-model="pcapForm.name" placeholder="请输入名称" />
      </el-form-item>
      <el-form-item label="* 团队">
        <el-select v-model="pcapForm.team" placeholder="请选择团队">
          <el-option label="Default团队" value="Default团队" />
        </el-select>
      </el-form-item>
      <el-form-item label="响应时间">
        <el-input v-model="pcapForm.description" placeholder="可输入55个任意字符" type="textarea" />
      </el-form-item>
      <el-form-item label="账号">
        <el-select v-model="pcapForm.account" placeholder="请选择">
          <el-option label="测试账号" value="test" />
        </el-select>
      </el-form-item>
      <el-form-item label="* 关联PCAP策略">
        <el-select v-model="pcapForm.pcapPolicy" placeholder="请选择">
          <el-option label="PCAP策略1" value="PCAP策略1" />
          <el-option label="PCAP策略2" value="PCAP策略2" />
        </el-select>
      </el-form-item>
      <el-form-item label="启用PCAP策略">
        <el-select v-model="pcapForm.enablePcap" placeholder="请选择">
          <el-option label="致命, 错误, 警告" value="fatal,error,warning" />
        </el-select>
      </el-form-item>
      <el-form-item label="禁用PCAP策略">
        <el-select v-model="pcapForm.disablePcap" placeholder="请选择">
          <el-option label="恢复" value="recovery" />
        </el-select>
      </el-form-item>
      <el-form-item label="* 推送周期">
        <el-select v-model="pcapForm.cycle" placeholder="请选择">
          <el-option label="15分钟" value="15min" />
          <el-option label="30分钟" value="30min" />
          <el-option label="1小时" value="1小时" />
        </el-select>
      </el-form-item>
      <el-form-item label="推送频率">
        <div class="frequency-control">
          <el-switch v-model="pcapForm.frequencyEnabled" />
          <el-input v-model="pcapForm.frequency" style="width: 80px; margin-left: 10px;" />
          <span style="margin-left: 10px;">秒</span>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="handleClose(false)">取消</el-button>
        <el-button type="primary" @click="handleConfirm">确定</el-button>
      </span>
    </template>
  </el-dialog>

  <!-- Syslog 推送表单 -->
  <el-dialog
    :model-value="visible && type === 'syslog'"
    @update:model-value="handleClose"
    title="新建连接Syslog推送"
    width="500px"
  >
    <el-form :model="syslogForm" label-width="100px">
      <el-form-item label="* 策略">
        <el-input v-model="syslogForm.name" placeholder="请输入名称" />
      </el-form-item>
      <el-form-item label="* 团队">
        <el-select v-model="syslogForm.team" placeholder="请选择团队">
          <el-option label="Default团队" value="Default团队" />
        </el-select>
      </el-form-item>
      <el-form-item label="响应时间">
        <el-input v-model="syslogForm.description" placeholder="可输入55个任意字符" type="textarea" />
      </el-form-item>
      <el-form-item label="* 推送目的端">
        <el-input v-model="syslogForm.destination" placeholder="[webhook://[host]:[port]" />
      </el-form-item>
      <el-form-item label="Message">
        <el-input v-model="syslogForm.message" placeholder="支持Jinja模板渲染，默认推送内容参考参数说明" type="textarea" />
        <div class="form-help">
          <el-button @click="showHelp">参数说明</el-button>
          <el-button @click="previewContent">预览</el-button>
        </div>
      </el-form-item>
      <el-form-item label="配置等级">
        <el-select v-model="syslogForm.level" placeholder="请选择">
          <el-option label="严重, 错误, 警告, 恢复, 未确认" value="fatal,error,warning,recovery,no_data" />
        </el-select>
      </el-form-item>
      <el-form-item label="* 推送周期">
        <el-select v-model="syslogForm.cycle" placeholder="请选择">
          <el-option label="15分钟" value="15min" />
          <el-option label="30分钟" value="30min" />
          <el-option label="1小时" value="1小时" />
        </el-select>
      </el-form-item>
      <el-form-item label="推送频率">
        <div class="frequency-control">
          <el-switch v-model="syslogForm.frequencyEnabled" />
          <el-input v-model="syslogForm.frequency" style="width: 80px; margin-left: 10px;" />
          <span style="margin-left: 10px;">秒</span>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="handleClose(false)">取消</el-button>
        <el-button type="primary" @click="handleConfirm">确定</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

// 定义 Props
const props = defineProps<{
  visible: boolean
  type: string
}>()

// 定义事件
const emit = defineEmits<{
  (e: 'confirm', form: any): void
  (e: 'cancel'): void
  (e: 'update:visible', value: boolean): void
}>()

// Email 表单
const emailForm = ref({
  name: '',
  team: 'Default团队',
  description: '',
  email: '',
  subject: '',
  level: 'fatal,error,warning,recovery,no_data',
  cycle: '15min',
  frequencyEnabled: false,
  frequency: 0
})

// HTTP 表单
const httpForm = ref({
  name: '',
  team: 'Default团队',
  description: '',
  method: 'POST',
  url: '',
  headerKey: '',
  headerValue: '',
  content: '',
  level: 'fatal,error,warning,recovery,no_data',
  cycle: '15min',
  frequencyEnabled: false,
  frequency: 0
})

// Kafka 表单
const kafkaForm = ref({
  name: '',
  team: 'Default团队',
  description: '',
  broker: '',
  topic: '',
  sasl: 'none',
  content: '',
  level: 'fatal,error,warning,recovery,no_data',
  cycle: '15min',
  frequencyEnabled: false,
  frequency: 0
})

// PCAP 表单
const pcapForm = ref({
  name: '',
  team: 'Default团队',
  description: '',
  account: '',
  pcapPolicy: '',
  enablePcap: 'fatal,error,warning',
  disablePcap: 'recovery',
  cycle: '15min',
  frequencyEnabled: false,
  frequency: 0
})

// Syslog 表单
const syslogForm = ref({
  name: '',
  team: 'Default团队',
  description: '',
  destination: '',
  message: '',
  level: 'fatal,error,warning,recovery,no_data',
  cycle: '15min',
  frequencyEnabled: false,
  frequency: 0
})

// 获取当前表单
const getCurrentForm = () => {
  switch (props.type) {
    case 'email':
      return emailForm.value
    case 'http':
      return httpForm.value
    case 'kafka':
      return kafkaForm.value
    case 'pcap':
      return pcapForm.value
    case 'syslog':
      return syslogForm.value
    default:
      return null
  }
}

// 关闭对话框
const handleClose = (value: boolean) => {
  emit('update:visible', value)
  if (!value) {
    emit('cancel')
  }
}

// 确认表单
const handleConfirm = () => {
  const form = getCurrentForm()
  if (form) {
    emit('confirm', { ...form, type: props.type })
  }
  handleClose(false)
}

// 测试表单
const testForm = () => {
  ElMessage.info('功能开发中...')
}

// 显示帮助
const showHelp = () => {
  ElMessage.info('功能开发中...')
}

// 预览内容
const previewContent = () => {
  ElMessage.info('功能开发中...')
}
</script>

<style scoped>
.frequency-control {
  display: flex;
  align-items: center;
}

.form-help {
  display: flex;
  gap: 10px;
  margin-top: 5px;
}

.header-control {
  display: flex;
  align-items: center;
}
</style>
