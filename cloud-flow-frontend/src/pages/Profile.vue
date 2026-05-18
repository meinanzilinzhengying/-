<template>
  <div class="profile-container">
    <el-card class="mb-4">
      <template #header>
        <div class="card-header">
          <h2>个人设置</h2>
        </div>
      </template>
      
      <div class="profile-content">
 <!-- 基本信息 -->
        <el-card class="mb-4">
          <template #header>
            <div class="card-header">
              <h3>基本信息</h3>
            </div>
          </template>
          <div class="basic-info">
            <div class="info-item">
              <span class="label">用户名：</span>
              <span class="value">{{ username }}</span>
            </div>
            <div class="info-item">
              <span class="label">邮箱：</span>
              <span class="value">{{ email }}</span>
            </div>
            <div class="info-item">
              <span class="label">角色：</span>
              <span class="value">{{ role }}</span>
            </div>
            <div class="info-item">
              <span class="label">最后登录时间：</span>
              <span class="value">{{ lastLoginTime }}</span>
            </div>
          </div>
        </el-card>
        
 <!-- 修改密码 -->
        <el-card class="mb-4">
          <template #header>
            <div class="card-header">
              <h3>修改密码</h3>
            </div>
          </template>
          <div class="password-form">
            <el-form :model="passwordForm" label-width="120px">
              <el-form-item label="当前密码">
                <el-input v-model="passwordForm.currentPassword" type="password" placeholder="请输入当前密码" />
              </el-form-item>
              <el-form-item label="新密码">
                <el-input v-model="passwordForm.newPassword" type="password" placeholder="请输入新密码" />
              </el-form-item>
              <el-form-item label="确认新密码">
                <el-input v-model="passwordForm.confirmPassword" type="password" placeholder="请确认新密码" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="updatePassword">修改密码</el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-card>
        
 <!-- 偏好设置 -->
        <el-card class="mb-4">
          <template #header>
            <div class="card-header">
              <h3>偏好设置</h3>
            </div>
          </template>
          <div class="preference-settings">
            <el-form :model="preferenceForm" label-width="120px">
              <el-form-item label="主题">
                <el-select v-model="preferenceForm.theme" placeholder="选择主题">
                  <el-option label="默认" value="default" />
                  <el-option label="暗色" value="dark" />
                </el-select>
              </el-form-item>
              <el-form-item label="语言">
                <el-select v-model="preferenceForm.language" placeholder="选择语言">
                  <el-option label="中文" value="zh-CN" />
                  <el-option label="英文" value="en-US" />
                </el-select>
              </el-form-item>
              <el-form-item label="时区">
                <el-select v-model="preferenceForm.timezone" placeholder="选择时区">
                  <el-option label="中国标准时间 (UTC+8)" value="Asia/Shanghai" />
                  <el-option label="美国东部时间 (UTC-5)" value="America/New_York" />
                  <el-option label="格林威治标准时间 (UTC)" value="GMT" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="savePreferences">保存设置</el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-card>
        
 <!-- 通知设置 -->
        <el-card class="mb-4">
          <template #header>
            <div class="card-header">
              <h3>通知设置</h3>
            </div>
          </template>
          <div class="notification-settings">
            <el-form :model="notificationForm" label-width="120px">
              <el-form-item label="邮件通知">
                <el-switch v-model="notificationForm.email" />
              </el-form-item>
              <el-form-item label="站内通知">
                <el-switch v-model="notificationForm.site" />
              </el-form-item>
              <el-form-item label="告警通知">
                <el-switch v-model="notificationForm.alert" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="saveNotifications">保存设置</el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-card>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue';
import { ElMessage } from 'element-plus';
import { useUserStore } from '../stores/user';
import { api } from '../utils/api';

const userStore = useUserStore();

const username = computed(() => userStore.userInfo?.username || 'admin');
const email = computed(() => userStore.userInfo?.email || '');
const role = computed(() => userStore.userInfo?.role || '管理员');

// 最后登录时间
const lastLoginTime = ref(new Date().toLocaleString());

// 密码表单
const passwordForm = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: ''
});

// 加载状态
const isPasswordUpdating = ref(false);

// 偏好设置表单
const preferenceForm = reactive({
  theme: 'default',
  language: 'zh-CN',
  timezone: 'Asia/Shanghai'
});

// 通知设置表单
const notificationForm = reactive({
  email: true,
  site: true,
  alert: true
});

// 方法
const updatePassword = async () => {
  if (!passwordForm.currentPassword) {
    ElMessage.warning('请输入当前密码');
    return;
  }
  if (!passwordForm.newPassword) {
    ElMessage.warning('请输入新密码');
    return;
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    ElMessage.warning('两次输入的密码不一致');
    return;
  }

  isPasswordUpdating.value = true;
  try {
    await userStore.changePassword(passwordForm.currentPassword, passwordForm.newPassword);
    ElMessage.success('密码修改成功');
 // 重置表单
    passwordForm.currentPassword = '';
    passwordForm.newPassword = '';
    passwordForm.confirmPassword = '';
  } catch (e) {
 // console.error('修改密码失败:', e);
    ElMessage.error('密码修改失败，请检查当前密码是否正确');
  } finally {
    isPasswordUpdating.value = false;
  }
};

const savePreferences = async () => {
  try {
    await api.user.updateUserInfo({ preferences: preferenceForm });
    ElMessage.success('偏好设置保存成功');
  } catch (e) {
    ElMessage.error('偏好设置保存失败');
  }
};

const saveNotifications = async () => {
  try {
    await api.user.updateUserInfo({ notifications: notificationForm });
    ElMessage.success('通知设置保存成功');
  } catch (e) {
    ElMessage.error('通知设置保存失败');
  }
};
</script>

<style scoped>
.profile-container {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

.card-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: bold;
  color: #303133;
}

.profile-content {
  margin-top: 20px;
}

.basic-info {
  padding: 20px;
  background-color: #f5f7fa;
  border-radius: 4px;
}

.info-item {
  margin-bottom: 15px;
  display: flex;
  align-items: center;
}

.info-item:last-child {
  margin-bottom: 0;
}

.label {
  width: 100px;
  font-weight: bold;
  color: #606266;
}

.value {
  color: #303133;
}

.password-form,
.preference-settings,
.notification-settings {
  padding: 20px;
}

</style>