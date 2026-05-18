<template>
  <div class="account-management">
 <!-- 账号信息 -->
    <div class="account-info">
      <h3>账号信息</h3>
      <el-button type="primary" size="small" @click="isEditing = !isEditing" style="margin-bottom: 20px;">{{ isEditing ? '取消' : '编辑' }}</el-button>

      <el-form :model="accountForm" :rules="rules" ref="accountFormRef" label-width="80px">
        <el-form-item label="UUID" prop="uuid">
          <el-input v-model="accountForm.uuid" :disabled="!isEditing" />
        </el-form-item>

        <el-form-item label="名称" prop="name">
          <el-input v-model="accountForm.name" :disabled="!isEditing" />
        </el-form-item>

        <el-form-item>
          <el-checkbox v-model="accountForm.changePassword">修改密码</el-checkbox>
        </el-form-item>

        <el-form-item v-if="accountForm.changePassword" label="原密码" prop="oldPassword">
          <el-input v-model="accountForm.oldPassword" type="password" />
        </el-form-item>

        <el-form-item v-if="accountForm.changePassword" label="密码" prop="password">
          <el-input v-model="accountForm.password" type="password" />
        </el-form-item>

        <el-form-item v-if="accountForm.changePassword" label="确认密码" prop="confirmPassword">
          <el-input v-model="accountForm.confirmPassword" type="password" />
        </el-form-item>

        <el-form-item label="部门">
          <el-input v-model="accountForm.department" :disabled="!isEditing" />
        </el-form-item>

        <el-form-item label="邮箱" prop="email">
          <el-input v-model="accountForm.email" :disabled="!isEditing" />
        </el-form-item>

        <el-form-item label="手机">
          <el-input v-model="accountForm.phone" :disabled="!isEditing" />
        </el-form-item>

        <el-form-item v-if="isEditing">
          <el-button type="primary" @click="submitForm">保存</el-button>
          <el-button @click="resetForm">重置</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const isEditing = ref(false)

const accountForm = ref({
  uuid: '550e8400-e29b-41d4-a716-446655440000',
  name: 'admin',
  department: '技术部',
  email: 'admin@example.com',
  phone: '13800138000',
  changePassword: false,
  oldPassword: '',
  password: '',
  confirmPassword: ''
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' }
  ]
}

const accountFormRef = ref(null)

const submitForm = () => {
  if (!accountForm.value.name) {
    ElMessage.warning('请输入名称')
    return
  }
  if (!accountForm.value.email) {
    ElMessage.warning('请输入邮箱')
    return
  }
  ElMessage.success('账号信息保存成功')
  isEditing.value = false
}

const resetForm = () => {
  accountForm.value = {
    uuid: '550e8400-e29b-41d4-a716-446655440000',
    name: 'admin',
    department: '技术部',
    email: 'admin@example.com',
    phone: '13800138000',
    changePassword: false,
    oldPassword: '',
    password: '',
    confirmPassword: ''
  }
  ElMessage.info('表单已重置')
}

const copyUUID = () => {
  navigator.clipboard.writeText(accountForm.value.uuid).then(() => {
    ElMessage.success('复制成功')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}
</script>

<style scoped>
.account-management {
  padding: 20px;
}
</style>