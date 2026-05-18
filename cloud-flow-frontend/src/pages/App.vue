<template>
  <div class="app-container">
 <!-- 子路由内容 -->
    <router-view />
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped>
.app-container {
  padding: 20px;
}

.app-content {
  background-color: white;
  border-radius: 4px;
  padding: 20px;
  min-height: 500px;
}

.app-content h2 {
  margin-top: 0;
  margin-bottom: 20px;
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

.app-content p {
  color: #909399;
  font-size: 14px;
}
</style>