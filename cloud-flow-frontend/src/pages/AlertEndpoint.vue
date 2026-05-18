<template>
  <div class="page-container">
    <h2>推送端点</h2>
    <p>推送端点管理页面</p>
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
