<template>
  <div class="system">
 <!-- 系统管理 -->
    <el-card class="mb-4">
      <template #header>
        <div class="card-header">
          <h2>系统管理</h2>
        </div>
      </template>

 <!-- 标签页 -->
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
 <!-- 采集器管理标签页 -->
        <el-tab-pane label="采集器管理" name="collector">
          <CollectorManagement />
        </el-tab-pane>
 <!-- 数据库节点管理标签页 -->
        <el-tab-pane label="数据库节点" name="data-node">
          <DataNodeManagement />
        </el-tab-pane>
 <!-- 账号管理标签页 -->
        <el-tab-pane label="账号管理" name="account">
          <AccountManagement />
        </el-tab-pane>
 <!-- 操作日志标签页 -->
        <el-tab-pane label="操作日志" name="log">
          <SystemLog />
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CollectorManagement, DataNodeManagement, AccountManagement, SystemLog } from './system'

const route = useRoute()
const router = useRouter()

const activeTab = computed({
  get: () => {
    const path = route.path.split('/').pop() || 'collector'
    return path
  },
  set: (value) => {
    router.push(`/system/${value}`)
  }
})

const handleTabChange = (tabName: string) => {
  router.push(`/system/${tabName}`)
}

watch(() => route.path, () => {
  const path = route.path.split('/').pop() || 'collector'
}, { immediate: true })
</script>

<style scoped>
.system {
  padding: 20px;
}
</style>