<template>

  <div class="topology-analysis-content">

 <!-- TCP重传详情-->

    <div class="topology-header">

      <div class="topology-controls">

        <el-form :inline="true" :model="topologyForm" class="demo-form-inline">

          <el-form-item label="系统日志源">

            <el-select v-model="topologyForm.dataSource" placeholder="选择数据源" style="width: 150px;">

              <el-option label="DeepFlow Statistics" value="deepflow" />

              <el-option label="网络设备" value="network" />

              <el-option label="应用调用" value="application" />

            </el-select>

          </el-form-item>

          <el-form-item label="命名空间">

            <el-select v-model="topologyForm.namespace" placeholder="default" style="width: 150px;">

              <el-option label="default" value="default" />

              <el-option label="kube-system" value="kube-system" />

              <el-option label="app" value="app" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="topologyForm.search" placeholder="搜索关键词" style="width: 200px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchTopology">

              <el-icon><Search /></el-icon> 搜索

            </el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="topology-actions">

        <el-form :inline="true" :model="topologyActionsForm" class="demo-form-inline">

          <el-form-item label="指标">

            <el-select v-model="topologyActionsForm.metric" placeholder="吞吐量" style="width: 120px;">

              <el-option label="吞吐量" value="throughput" />

              <el-option label="延迟" value="latency" />

              <el-option label="丢包率" value="packetLoss" />

            </el-select>

          </el-form-item>

          <el-form-item label="Topo">

            <el-select v-model="topologyActionsForm.topoType" placeholder="服务" style="width: 100px;">

              <el-option label="服务" value="service" />

              <el-option label="主机名" value="host" />

              <el-option label="容器" value="container" />

            </el-select>

          </el-form-item>

          <el-form-item label="请求时间">

            <el-select v-model="topologyActionsForm.timeRange" placeholder="5分钟" style="width: 100px;">

              <el-option label="5分钟" value="5m" />

              <el-option label="15分钟" value="15m" />

              <el-option label="30分钟" value="30m" />

              <el-option label="1小时" value="1h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-button @click="refreshTopology">

              <el-icon><Refresh /></el-icon> 刷新

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="exportTopology">

              <el-icon><Download /></el-icon> 导出数据库

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button type="danger" @click="clearTopology">

              <el-icon><Delete /></el-icon> 保存搜索

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 服务调用关系图-->

    <div class="topology-container">

 <!-- 节点和边的详情 -->

      <div class="topology-legend">

        <h4>图例</h4>

        <div class="legend-item">

          <span class="legend-color service-color"></span>

          <span>服务</span>

        </div>

        <div class="legend-item">

          <span class="legend-color database-color"></span>

          <span>拓扑图展示</span>

        </div>

        <div class="legend-item">

          <span class="legend-color cache-color"></span>

          <span>网络连接</span>

        </div>

        <div class="legend-item">

          <span class="legend-color message-color"></span>

          <span>链路追踪服务发现</span>

        </div>

        <h4>流量类型</h4>

        <div class="legend-item">

          <span class="legend-line normal-line"></span>

          <span>正常流量</span>

        </div>

        <div class="legend-item">

          <span class="legend-line warning-line"></span>

          <span>警告流量</span>

        </div>

        <div class="legend-item">

          <span class="legend-line error-line"></span>

          <span>错误流量</span>

        </div>

      </div>

      

 <!-- 节点流量信息-->

      <div class="topology-graph">

        <div class="mock-topology">

 <!-- 拓扑图模型-->

          <div class="topology-node load-balancer">

            <div class="node-content">

              <div class="node-icon"><el-icon><Refresh /></el-icon></div>

              <div class="node-name">loadgenerator</div>

            </div>

          </div>

          

          <div class="topology-node frontend">

            <div class="node-content">

              <div class="node-icon"><el-icon><View /></el-icon></div>

              <div class="node-name">frontend</div>

            </div>

          </div>

          

          <div class="topology-node productcatalog">

            <div class="node-content">

              <div class="node-icon"><el-icon><Document /></el-icon></div>

              <div class="node-name">productcatalog</div>

            </div>

          </div>

          

          <div class="topology-node currencyservice">

            <div class="node-content">

              <div class="node-icon"><el-icon><Document /></el-icon></div>

              <div class="node-name">currencyservice</div>

            </div>

          </div>

          

          <div class="topology-node cartservice">

            <div class="node-content">

              <div class="node-icon"><el-icon><Document /></el-icon></div>

              <div class="node-name">cartservice</div>

            </div>

          </div>

          

          <div class="topology-node recommendations">

            <div class="node-content">

              <div class="node-icon"><el-icon><Document /></el-icon></div>

              <div class="node-name">recommendations</div>

            </div>

          </div>

          

          <div class="topology-node adservice">

            <div class="node-content">

              <div class="node-icon"><el-icon><Document /></el-icon></div>

              <div class="node-name">adservice</div>

            </div>

          </div>

          

          <div class="topology-node shippingservice">

            <div class="node-content">

              <div class="node-icon"><el-icon><Document /></el-icon></div>

              <div class="node-name">shippingservice</div>

            </div>

          </div>

          

          <div class="topology-node checkoutservice">

            <div class="node-content">

              <div class="node-icon"><el-icon><Document /></el-icon></div>

              <div class="node-name">checkoutservice</div>

            </div>

          </div>

          

          <div class="topology-node redis-cart">

            <div class="node-content">

              <div class="node-icon"><el-icon><Cpu /></el-icon></div>

              <div class="node-name">redis-cart</div>

            </div>

          </div>

          

          <div class="topology-node emailservice">

            <div class="node-content">

              <div class="node-icon"><el-icon><Message /></el-icon></div>

              <div class="node-name">emailservice</div>

            </div>

          </div>

          

          <div class="topology-node paymentservice">

            <div class="node-content">

              <div class="node-icon"><el-icon><Document /></el-icon></div>

              <div class="node-name">paymentservice</div>

            </div>

          </div>

          

 <!-- 端口支持详情-->

          <div class="topology-links">

            <div class="link link-1"></div>

            <div class="link link-2"></div>

            <div class="link link-3"></div>

            <div class="link link-4"></div>

            <div class="link link-5"></div>

            <div class="link link-6"></div>

            <div class="link link-7"></div>

            <div class="link link-8"></div>

            <div class="link link-9"></div>

            <div class="link link-10"></div>

            <div class="link link-11"></div>

          </div>

        </div>

      </div>

      

 <!-- 时间范围选择器 -->

      <div class="topology-controls-panel">

        <h4>控制面板</h4>

        <el-button @click="zoomIn">放大</el-button>

        <el-button @click="zoomOut">缩小</el-button>

        <el-button @click="resetView">重置视图</el-button>

        <el-button @click="toggleLabels">显示/隐藏标签</el-button>

        <el-button @click="toggleTraffic">显示名称/隐藏流量</el-button>

        <h4>节点流量信息</h4>

        <div class="node-info">

          <p>点击节点查看详细信息</p>

        </div>

      </div>

    </div>

  </div>

</template>



<script setup lang="ts">

import { reactive } from 'vue';

import { Search, Refresh, Download, Delete, View, Cpu, Message, Document } from '@element-plus/icons-vue';



// 表单流量详情

const topologyForm = reactive({

  dataSource: 'deepflow',

  namespace: 'default',

  search: ''

});



const topologyActionsForm = reactive({

  metric: 'throughput',

  topoType: 'service',

  timeRange: '5m'

});



// 新建规则

const searchTopology = () => {

  };



const refreshTopology = () => {

  };



const exportTopology = () => {

  };



const clearTopology = () => {

  };



const zoomIn = () => {

  };



const zoomOut = () => {

  };



const resetView = () => {

  };



const toggleLabels = () => {

  };



const toggleTraffic = () => {

  };

</script>



<style scoped>

.topology-analysis-content {

  padding: 20px;

}



.topology-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.topology-controls {

  display: flex;

  align-items: center;

}



.topology-actions {

  display: flex;

  align-items: center;

}



.topology-container {

  display: flex;

  height: 600px;

  margin-top: 20px;

}



.topology-legend {

  width: 200px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

  margin-right: 20px;

  overflow-y: auto;

}



.topology-legend h4 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 14px;

  font-weight: bold;

}



.legend-item {

  display: flex;

  align-items: center;

  margin-bottom: 8px;

  font-size: 12px;

}



.legend-color {

  width: 12px;

  height: 12px;

  border-radius: 50%;

  margin-right: 8px;

}



.service-color {

  background-color: #409eff;

}



.database-color {

  background-color: #67c23a;

}



.cache-color {

  background-color: #e6a23c;

}



.message-color {

  background-color: #f56c6c;

}



.legend-line {

  width: 30px;

  height: 2px;

  margin-right: 8px;

}



.normal-line {

  background-color: #67c23a;

}



.warning-line {

  background-color: #e6a23c;

}



.error-line {

  background-color: #f56c6c;

}



.topology-graph {

  flex: 1;

  background-color: white;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  position: relative;

  overflow: hidden;

}



.mock-topology {

  width: 100%;

  height: 100%;

  position: relative;

}



.topology-node {

  position: absolute;

  background-color: white;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  padding: 10px;

  min-width: 100px;

  text-align: center;

  cursor: pointer;

  transition: all 0.3s ease;

}



.topology-node:hover {

  transform: translateY(-2px);

  box-shadow: 0 4px 12px 0 rgba(0, 0, 0, 0.15);

}



.node-content {

  display: flex;

  flex-direction: column;

  align-items: center;

}



.node-icon {

  font-size: 24px;

  margin-bottom: 5px;

}



.node-name {

  font-size: 12px;

  white-space: nowrap;

  overflow: hidden;

  text-overflow: ellipsis;

  max-width: 100px;

}



/* 节点位置样式 */

.load-balancer {

  top: 50px;

  left: 50%;

  transform: translateX(-50%);

}



.frontend {

  top: 150px;

  left: 50%;

  transform: translateX(-50%);

}



.productcatalog {

  top: 250px;

  left: 15%;

}



.currencyservice {

  top: 250px;

  left: 30%;

}



.cartservice {

  top: 250px;

  left: 45%;

}



.recommendations {

  top: 250px;

  left: 60%;

}



.adservice {

  top: 250px;

  left: 75%;

}



.shippingservice {

  top: 350px;

  left: 75%;

}



.checkoutservice {

  top: 350px;

  left: 85%;

}



.redis-cart {

  top: 350px;

  left: 45%;

}



.emailservice {

  top: 450px;

  left: 60%;

}



.paymentservice {

  top: 450px;

  left: 75%;

}



/* 端口支持详情?*/

.topology-links {

  position: absolute;

  top: 0;

  left: 0;

  width: 100%;

  height: 100%;

  pointer-events: none;

}



.link {

  position: absolute;

  background-color: #dcdfe6;

  height: 2px;

  transform-origin: left center;

}



.link-1 {

  top: 100px;

  left: 50%;

  width: 10px;

  transform: rotate(0deg);

}



.link-2 {

  top: 200px;

  left: 50%;

  width: 150px;

  transform: rotate(-45deg);

}



.link-3 {

  top: 200px;

  left: 50%;

  width: 100px;

  transform: rotate(-25deg);

}



.link-4 {

  top: 200px;

  left: 50%;

  width: 50px;

  transform: rotate(0deg);

}



.link-5 {

  top: 200px;

  left: 50%;

  width: 100px;

  transform: rotate(25deg);

}



.link-6 {

  top: 200px;

  left: 50%;

  width: 150px;

  transform: rotate(45deg);

}



.link-7 {

  top: 300px;

  left: 75%;

  width: 50px;

  transform: rotate(0deg);

}



.link-8 {

  top: 300px;

  left: 85%;

  width: 30px;

  transform: rotate(0deg);

}



.link-9 {

  top: 300px;

  left: 45%;

  width: 50px;

  transform: rotate(0deg);

}



.link-10 {

  top: 400px;

  left: 60%;

  width: 50px;

  transform: rotate(25deg);

}



.link-11 {

  top: 400px;

  left: 75%;

  width: 30px;

  transform: rotate(0deg);

}



.topology-controls-panel {

  width: 200px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

  margin-left: 20px;

  overflow-y: auto;

}



.topology-controls-panel h4 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 14px;

  font-weight: bold;

}



.topology-controls-panel .el-button {

  width: 100%;

  margin-bottom: 8px;

}



.node-info {

  margin-top: 20px;

  padding: 10px;

  background-color: white;

  border-radius: 4px;

  font-size: 12px;

  color: #606266;

}

</style>