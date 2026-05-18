<template>
  <div class="page-container">
    <h2>告警策略</h2>
    <p>告警策略管理页面</p>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { ElMessage } from 'element-plus'

onMounted(() => {
  ElMessage.info('功能开发中...')
})
</script>

<style scoped>
.page-container {
  padding: 20px;
}
</style>
