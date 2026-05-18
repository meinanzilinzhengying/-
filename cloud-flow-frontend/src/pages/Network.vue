<template>
  <div class="network-container">
 <!-- 网络观测 -->
    <el-card class="mb-4">
      <template #header>
        <div class="card-header">
          <h2>{{ currentPageTitle }}</h2>
        </div>
      </template>
      
 <!-- 资源分析页面 -->
      <ResourceAnalysis v-if="currentPage === 'resource'" />
      
 <!-- 路径分析页面 -->
      <PathAnalysis v-else-if="currentPage === 'path'" />
      
 <!-- 拓扑分析页面 -->
      <TopologyAnalysis v-else-if="currentPage === 'topology'" />
      
 <!-- 流日志页面 -->
      <FlowAnalysis v-else-if="currentPage === 'flow'" />

 <!-- NAT追踪页面 -->
      <NATAnalysis v-else-if="currentPage === 'nat'" />

 <!-- PCAP策略页面 -->
      <PCAPStrategy v-else-if="currentPage === 'pcap'" />

 <!-- PCAP下载页面 -->
      <PCAPDownload v-else-if="currentPage === 'pcap-download'" />

 <!-- 流量分发页面 -->
      <FlowDistribution v-else-if="currentPage === 'distribution'" />

 <!-- 资源盘点页面 -->
      <ResourceInventory v-else-if="currentPage === 'inventory'" />

 <!-- 其他页面 -->
      <div v-else class="empty-page">
        <el-empty description="页面开发中" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import ResourceAnalysis from '../components/network/ResourceAnalysis.vue';
import PathAnalysis from '../components/network/PathAnalysis.vue';
import TopologyAnalysis from '../components/network/TopologyAnalysis.vue';
import FlowAnalysis from '../components/network/FlowAnalysis.vue';
import NATAnalysis from '../components/network/NATAnalysis.vue';
import PCAPStrategy from '../components/network/PCAPStrategy.vue';
import PCAPDownload from '../components/network/PCAPDownload.vue';
import FlowDistribution from '../components/network/FlowDistribution.vue';
import ResourceInventory from '../components/network/ResourceInventory.vue';

const route = useRoute();

// 当前页面 - 响应路由变化
const currentPage = computed(() => {
  const path = route.path.split('/').pop() || 'resource';
  return path;
});

// 当前页面标题
const currentPageTitle = computed(() => {
  const titles: Record<string, string> = {
    resource: '资源分析',
    path: '路径分析',
    topology: '拓扑分析',
    flow: '流日志',
    nat: 'NAT追踪',
    pcap: 'PCAP策略',
    'pcap-download': 'PCAP下载',
    distribution: '流量分发',
    inventory: '资源盘点'
  };
  return titles[currentPage.value] || '网络观测';
});
</script>

<style scoped>
.network-container {
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

.empty-page {
  padding: 60px 20px;
  text-align: center;
}
</style>