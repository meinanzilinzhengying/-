<template>

  <div class="business-topology">

    <div class="topology-header">

      <h3>请求链路追踪</h3>

      <div class="header-actions">

        <el-form :inline="true" :model="topologyForm" class="demo-form-inline">

          <el-form-item>

            <el-select v-model="topologyForm.business" placeholder="选择业务" style="width: 150px;">

              <el-option label="全部" value="" />

              <el-option label="电商业务" value="电商业务" />

              <el-option label="物流业务" value="物流业务" />

              <el-option label="营销业务" value="营销业务" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="loadTopology">

              服务调用拓扑

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-dropdown>

              <el-button>

                布局

                <el-icon class="el-icon--right"><ArrowDown /></el-icon>

              </el-button>

              <template #dropdown>

                <el-dropdown-menu>

                  <el-dropdown-item @click="layoutTopology('circular')">环形布局</el-dropdown-item>

                  <el-dropdown-item @click="layoutTopology('force')">力导向布局</el-dropdown-item>

                  <el-dropdown-item @click="layoutTopology('hierarchical')">层级布局</el-dropdown-item>

                </el-dropdown-menu>

              </template>

            </el-dropdown>

          </el-form-item>

        </el-form>

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

                <div class="metric-item">分组聚合详情: 2.33ms</div>

              </div>

            </div>

          </div>

          <div class="topology-node">

            <div class="node-content">

              <div class="node-label">svc-user</div>

              <div class="node-metrics">

                <div class="metric-item">QPS: 800</div>

                <div class="metric-item">分组聚合详情: 1.2ms</div>

              </div>

            </div>

          </div>

          <div class="topology-node">

            <div class="node-content">

              <div class="node-label">svc-order</div>

              <div class="node-metrics">

                <div class="metric-item">QPS: 600</div>

                <div class="metric-item">分组聚合详情: 1.5ms</div>

              </div>

            </div>

          </div>

          <div class="topology-node">

            <div class="node-content">

              <div class="node-label">svc-payment</div>

              <div class="node-metrics">

                <div class="metric-item">QPS: 500</div>

                <div class="metric-item">分组聚合详情: 2.0ms</div>

              </div>

            </div>

          </div>

          <div class="topology-node">

            <div class="node-content">

              <div class="node-label">svc-shipping</div>

              <div class="node-metrics">

                <div class="metric-item">QPS: 400</div>

                <div class="metric-item">分组聚合详情: 1.8ms</div>

              </div>

            </div>

          </div>

          <div class="topology-node">

            <div class="node-content">

              <div class="node-label">svc-warehouse</div>

              <div class="node-metrics">

                <div class="metric-item">QPS: 300</div>

                <div class="metric-item">分组聚合详情: 1.3ms</div>

              </div>

            </div>

          </div>

          <div class="topology-node">

            <div class="node-content">

              <div class="node-label">svc-marketing</div>

              <div class="node-metrics">

                <div class="metric-item">QPS: 200</div>

                <div class="metric-item">分组聚合详情: 1.0ms</div>

              </div>

            </div>

          </div>

          <div class="topology-node">

            <div class="node-content">

              <div class="node-label">svc-coupon</div>

              <div class="node-metrics">

                <div class="metric-item">QPS: 100</div>

                <div class="metric-item">分组聚合详情: 0.8ms</div>

              </div>

            </div>

          </div>

          <svg class="topology-connections" width="100%" height="100%">

            <line x1="100" y1="150" x2="200" y2="100" stroke="#409eff" stroke-width="2" />

            <line x1="100" y1="150" x2="200" y2="200" stroke="#409eff" stroke-width="2" />

            <line x1="100" y1="150" x2="300" y2="150" stroke="#409eff" stroke-width="2" />

            <line x1="200" y1="100" x2="300" y2="50" stroke="#409eff" stroke-width="2" />

            <line x1="200" y1="200" x2="300" y2="250" stroke="#409eff" stroke-width="2" />

            <line x1="300" y1="150" x2="400" y2="100" stroke="#409eff" stroke-width="2" />

            <line x1="300" y1="150" x2="400" y2="200" stroke="#409eff" stroke-width="2" />

          </svg>

        </div>

      </div>

      <div class="topology-info">

        <h4>节点流量信息</h4>

        <el-table :data="topologyData" style="width: 100%">

          <el-table-column prop="source" label="源服务名称" width="150" />

          <el-table-column prop="target" label="目标服务" width="150" />

          <el-table-column prop="requestRate" label="网络流量监控" width="120" />

          <el-table-column prop="errorRate" label="错误率" width="100" />

          <el-table-column prop="responseTime" label="分组聚合详情" width="120" />

        </el-table>

      </div>

    </div>

  </div>

</template>



<script setup lang="ts">

import { ref } from 'vue'

import { ArrowDown } from '@element-plus/icons-vue'



// 节点流量信息功能

const topologyForm = ref({

  business: ''

})



// 拓扑数据库流

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

    source: 'web-shop',

    target: 'svc-payment',

    requestRate: '1.5',

    errorRate: '0%',

    responseTime: '2.0 ms'

  },

  {

    source: 'svc-order',

    target: 'svc-shipping',

    requestRate: '1.2',

    errorRate: '0%',

    responseTime: '1.8 ms'

  },

  {

    source: 'svc-shipping',

    target: 'svc-warehouse',

    requestRate: '0.9',

    errorRate: '0%',

    responseTime: '1.3 ms'

  },

  {

    source: 'web-shop',

    target: 'svc-marketing',

    requestRate: '0.7',

    errorRate: '0%',

    responseTime: '1.0 ms'

  },

  {

    source: 'svc-marketing',

    target: 'svc-coupon',

    requestRate: '0.5',

    errorRate: '0%',

    responseTime: '0.8 ms'

  }

])



// 服务调用拓扑

const loadTopology = () => {

 // 模板加载数据

}



// 布局拓扑

const layoutTopology = (layout: string) => {

 // 模拟布局

}

</script>



<style scoped>

.business-topology {

  padding: 20px;

}



.topology-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

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

  gap: 20px;

}



.topology-chart {

  flex: 2;

  background-color: white;

  border-radius: 4px;

  padding: 20px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.topology-info {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 20px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.topology-info h4 {

  margin-top: 0;

  margin-bottom: 15px;

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

  padding: 20px;

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

  top: 50px;

  left: 300px;

}



.topology-node:nth-child(6) {

  top: 250px;

  left: 300px;

}



.topology-node:nth-child(7) {

  top: 100px;

  left: 400px;

}



.topology-node:nth-child(8) {

  top: 200px;

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

</style>