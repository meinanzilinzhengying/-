<template>
  <div class="business-topology-view">
    <div class="topology-header">
      <h3>{{ businessName }} - 服务拓扑</h3>
      <div class="header-actions">
        <el-button @click="goBack">
          返回
        </el-button>
      </div>
    </div>
    <div class="topology-content">
      <div class="topology-chart">
        <div class="mock-topology">
          <div class="topology-node">
            <div class="node-content">
              <div class="node-label">web-shop</div>
              <div class="node-metrics">
                <div class="metric-item">QPS: 1000</div>
                <div class="metric-item">响应时间: 2.33ms</div>
                <div class="metric-item error" v-if="hasError('web-shop')">错误率: 5%</div>
              </div>
            </div>
          </div>
          <div class="topology-node">
            <div class="node-content">
              <div class="node-label">svc-user</div>
              <div class="node-metrics">
                <div class="metric-item">QPS: 800</div>
                <div class="metric-item">响应时间: 1.2ms</div>
              </div>
            </div>
          </div>
          <div class="topology-node">
            <div class="node-content">
              <div class="node-label">svc-order</div>
              <div class="node-metrics">
                <div class="metric-item">QPS: 600</div>
                <div class="metric-item">响应时间: 1.5ms</div>
              </div>
            </div>
          </div>
          <div class="topology-node">
            <div class="node-content">
              <div class="node-label">svc-payment</div>
              <div class="node-metrics">
                <div class="metric-item">QPS: 500</div>
                <div class="metric-item">响应时间: 2.0ms</div>
                <div class="metric-item error" v-if="hasError('svc-payment')">错误率: 3%</div>
              </div>
            </div>
          </div>
          <div class="topology-node">
            <div class="node-content">
              <div class="node-label">svc-shipping</div>
              <div class="node-metrics">
                <div class="metric-item">QPS: 400</div>
                <div class="metric-item">响应时间: 1.8ms</div>
              </div>
            </div>
          </div>
          <svg class="topology-connections" width="100%" height="100%">
            <line x1="100" y1="150" x2="200" y2="100" stroke="#409eff" stroke-width="2" />
            <line x1="100" y1="150" x2="200" y2="200" stroke="#409eff" stroke-width="2" />
            <line x1="200" y1="200" x2="300" y2="150" stroke="#409eff" stroke-width="2" />
            <line x1="300" y1="150" x2="400" y2="100" stroke="#409eff" stroke-width="2" />
          </svg>
        </div>
      </div>
      <div class="topology-info">
        <h4>拓扑信息</h4>
        <el-table :data="topologyData" style="width: 100%">
          <el-table-column prop="source" label="源服务" width="150" />
          <el-table-column prop="target" label="目标服务" width="150" />
          <el-table-column prop="requestRate" label="请求速率" width="120" />
          <el-table-column prop="errorRate" label="错误率" width="100" />
          <el-table-column prop="responseTime" label="响应时间" width="120" />
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'

const router = useRouter()
const route = useRoute()

// 业务名称
const businessName = ref('电商业务')

// 拓扑数据库
const topologyData = ref([
  {
    source: 'web-shop',
    target: 'svc-user',
    requestRate: '2.11',
    errorRate: '0%',
    responseTime: '1.2 ms'
  },
  {
    source: 'web-shop',
    target: 'svc-order',
    requestRate: '1.87',
    errorRate: '0%',
    responseTime: '1.5 ms'
  },
  {
    source: 'svc-order',
    target: 'svc-payment',
    requestRate: '1.5',
    errorRate: '3%',
    responseTime: '2.0 ms'
  },
  {
    source: 'svc-order',
    target: 'svc-shipping',
    requestRate: '1.2',
    errorRate: '0%',
    responseTime: '1.8 ms'
  }
])

// 检查服务是否有错误
const hasError = (service: string) => {
  return service === 'web-shop' || service === 'svc-payment'
}

// 返回
const goBack = () => {
  router.push('/business')
}
</script>

<style scoped>
.business-topology-view {
  padding: 24px;
}

.topology-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.topology-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: bold;
  color: #303133;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.topology-content {
  display: flex;
  gap: 24px;
}

.topology-chart {
  flex: 2;
  background-color: white;
  border-radius: 4px;
  padding: 24px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

.topology-info {
  flex: 1;
  background-color: white;
  border-radius: 4px;
  padding: 24px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

.topology-info h4 {
  margin-top: 0;
  margin-bottom: 24px;
  font-size: 14px;
  font-weight: bold;
  color: #303133;
}

.mock-topology {
  position: relative;
  width: 100%;
  height: 400px;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  padding: 24px;
}

.topology-node {
  position: absolute;
  cursor: pointer;
  transition: all 0.3s ease;
}

.topology-node:hover {
  transform: scale(1.05);
}

.topology-node:nth-child(1) {
  top: 150px;
  left: 100px;
}

.topology-node:nth-child(2) {
  top: 100px;
  left: 200px;
}

.topology-node:nth-child(3) {
  top: 200px;
  left: 200px;
}

.topology-node:nth-child(4) {
  top: 150px;
  left: 300px;
}

.topology-node:nth-child(5) {
  top: 100px;
  left: 400px;
}

.node-content {
  padding: 15px;
  background-color: white;
  border: 1px solid #409eff;
  border-radius: 4px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  text-align: center;
  min-width: 100px;
}

.node-label {
  font-size: 14px;
  font-weight: bold;
  color: #303133;
  margin-bottom: 10px;
}

.node-metrics {
  font-size: 12px;
  color: #606266;
}

.metric-item {
  margin-bottom: 5px;
}

.metric-item.error {
  color: #FF4D4F;
  font-weight: bold;
}

.topology-connections {
  position: absolute;
  top: 0;
  left: 0;
  z-index: 0;
  pointer-events: none;
}

@media (max-width: 1200px) {
  .topology-content {
    flex-direction: column;
  }
  
  .topology-chart,
  .topology-info {
    flex: 1;
  }
}

@media (max-width: 768px) {
  .topology-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  
  .mock-topology {
    height: 300px;
  }
  
  .topology-node {
    min-width: 80px;
  }
  
  .node-content {
    padding: 10px;
  }
  
  .node-label {
    font-size: 12px;
  }
  
  .node-metrics {
    font-size: 10px;
  }
}

:deep(.el-button--primary) {
  background-color: #1677FF;
  border-color: #1677FF;
}

:deep(.el-button--danger) {
  background-color: #FF4D4F;
  border-color: #FF4D4F;
}
</style>