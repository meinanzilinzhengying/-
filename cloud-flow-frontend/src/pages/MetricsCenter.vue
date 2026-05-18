<template>
  <div class="metrics-center">
    <!-- 指标中心 -->
    <el-card class="mb-4">
      <template #header>
        <div class="card-header">
          <h2>{{ currentPageTitle }}</h2>
        </div>
      </template>

      <!-- 主机页面 -->
      <HostTable v-if="currentPage === 'host'" />

      <!-- 容器页面 -->
      <ContainerTable v-else-if="currentPage === 'container'" />

      <!-- 数据库视图页面 -->
      <DatabaseView v-else-if="currentPage === 'view'" />

      <!-- 指标摘要页面 -->
      <MetricsSummary v-else-if="currentPage === 'summary'" />

      <!-- 指标模板页面 -->
      <MetricsTemplate v-else-if="currentPage === 'template'" />

    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'

// 子组件
import HostTable from '@/components/metrics/HostTable.vue'
import ContainerTable from '@/components/metrics/ContainerTable.vue'
import DatabaseView from '@/components/metrics/DatabaseView.vue'
import MetricsSummary from '@/components/metrics/MetricsSummary.vue'
import MetricsTemplate from '@/components/metrics/MetricsTemplate.vue'

// 路由
const route = useRoute()

// 响应路由变化
const currentPage = ref('host')

// 页面标题
const currentPageTitle = computed(() => {
  const pageTitles = {
    'host': '主机',
    'container': '容器',
    'view': '数据库视图',
    'summary': '指标摘要',
    'template': '指标模板'
  }
  return pageTitles[currentPage.value] || '指标中心'
})

// 加载状态
const loading = ref({
  hosts: false,
  containers: false,
  history: false,
  metrics: false
})

// 错误信息
const error = ref({
  hosts: '',
  containers: '',
  history: '',
  metrics: ''
})

// 根据路由设置当前页面
watch(() => route.path, (newPath) => {
  const pathSegments = newPath.split('/')
  if (pathSegments.length > 2 && pathSegments[1] === 'metrics-center') {
    currentPage.value = pathSegments[2]
  }
}, { immediate: true })
</script>

<style scoped>
.metrics-center {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
