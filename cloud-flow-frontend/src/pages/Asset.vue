<template>
  <div class="asset">
 <!-- 变更事件 -->
    <el-card v-if="currentModule === 'change-event'" class="mb-4">
      <ChangeEvent />
    </el-card>

 <!-- 资源池 -->
    <el-card v-else-if="currentModule === 'resource-pool'" class="mb-4">
      <ResourcePool />
    </el-card>

 <!-- 计算资源 -->
    <el-card v-else-if="currentModule === 'compute'" class="mb-4">
      <ComputeResource />
    </el-card>

 <!-- 网络资源 -->
    <el-card v-else-if="currentModule === 'network'" class="mb-4">
      <NetworkResource />
    </el-card>

 <!-- 网络服务 -->
    <el-card v-else-if="currentModule === 'network-service'" class="mb-4">
      <NetworkService />
    </el-card>

 <!-- 存储服务 -->
    <el-card v-else-if="currentModule === 'storage'" class="mb-4">
      <StorageResource />
    </el-card>

 <!-- 应用 -->
    <el-card v-else-if="currentModule === 'application'" class="mb-4">
      <div class="card-header">
        <h2>应用</h2>
      </div>
      <div class="empty-content">
        <el-empty description="页面开发中" />
      </div>
    </el-card>

 <!-- 容器 -->
    <el-card v-else-if="currentModule === 'container'" class="mb-4">
      <ContainerResource />
    </el-card>

 <!-- K8s集群 -->
    <el-card v-else-if="currentModule === 'k8s'" class="mb-4">
      <div class="card-header">
        <h2>K8s集群</h2>
      </div>
      <div class="empty-content">
        <el-empty description="页面开发中" />
      </div>
    </el-card>

 <!-- 进程资源 -->
    <el-card v-else-if="currentModule === 'process'" class="mb-4">
      <ProcessResource />
    </el-card>

 <!-- 其他资源 -->
    <el-card v-else-if="currentModule === 'other'" class="mb-4">
      <OtherResource />
    </el-card>

 <!-- 其他模块 -->
    <el-card v-else class="mb-4">
      <div class="card-header">
        <h2>404</h2>
      </div>
      <div class="empty-content">
        <el-empty description="页面不存在" />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import ChangeEvent from '../components/asset/ChangeEvent.vue';
import ResourcePool from '../components/asset/ResourcePool.vue';
import ComputeResource from '../components/asset/ComputeResource.vue';
import NetworkResource from '../components/asset/NetworkResource.vue';
import NetworkService from '../components/asset/NetworkService.vue';
import StorageResource from '../components/asset/StorageResource.vue';
import ContainerResource from '../components/asset/ContainerResource.vue';
import ProcessResource from '../components/asset/ProcessResource.vue';
import OtherResource from '../components/asset/OtherResource.vue';

// 当前模块
const route = useRoute();
const currentModule = computed(() => {
  const parts = route.path.split('/');
  return parts.pop() || 'change-event';
});
</script>

<style scoped>
.asset {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 15px;
  background-color: #f5f7fa;
  border-radius: 4px;
}

.card-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

.empty-content {
  padding: 60px 20px;
  text-align: center;
}

</style>