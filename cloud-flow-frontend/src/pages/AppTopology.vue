<template>

  <div class="app-topology-content">

 <!-- TCP重传详情-->

    <div class="topology-header">

      <div class="topology-search">

        <el-form :inline="true" :model="topologyForm" class="demo-form-inline">

          <el-form-item label="时间范围">

            <el-select v-model="topologyForm.snapshot" placeholder="查询快照" style="width: 200px;">

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="topologyForm.search" placeholder="搜索关键词" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchTopology">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="topology-actions">

        <el-form :inline="true" :model="topologyActionsForm" class="demo-form-inline">

          <el-form-item>

            <el-button @click="saveSearch">

              保存搜索条件

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-dropdown>

              <el-button>

                设置

                <el-icon class="el-icon--right"><ArrowDown /></el-icon>

              </el-button>

              <template #dropdown>

                <el-dropdown-menu>

                  <el-dropdown-item @click="showDatabaseFields">数据库字段</el-dropdown-item>

                  <el-dropdown-item @click="switchNameDisplay">切换名称显示/隐藏策略</el-dropdown-item>

                </el-dropdown-menu>

              </template>

            </el-dropdown>

          </el-form-item>

          <el-form-item>

            <el-button @click="refreshTopology">

              刷新

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="exportTopology">

              导出数据库

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 业务监控-->

    <div class="topology-content">

 <!-- 左侧快速过滤-->

      <div class="topology-sidebar">

 <!-- 应用服务 -->

        <div class="filter-section">

          <h3>应用服务</h3>

          <el-checkbox-group v-model="selectedApps">

            <el-checkbox label="frontend">frontend</el-checkbox>

            <el-checkbox label="productcatalogservice">productcatalogservice</el-checkbox>

            <el-checkbox label="currencyservice">currencyservice</el-checkbox>

            <el-checkbox label="cartservice">cartservice</el-checkbox>

            <el-checkbox label="recommendationservice">recommendationservice</el-checkbox>

            <el-checkbox label="adservice">adservice</el-checkbox>

            <el-checkbox label="shippingservice">shippingservice</el-checkbox>

            <el-checkbox label="checkoutservice">checkoutservice</el-checkbox>

            <el-checkbox label="redis-cart">redis-cart</el-checkbox>

            <el-checkbox label="emailservice">emailservice</el-checkbox>

            <el-checkbox label="paymentservice">paymentservice</el-checkbox>

          </el-checkbox-group>

        </div>

        

 <!-- 区域查询 -->

        <div class="filter-section">

          <h3>区域查询</h3>

          <el-radio-group v-model="regionQuery">

            <el-radio label="全部">全部</el-radio>

            <el-radio label="区域1">区域1</el-radio>

            <el-radio label="区域2">区域2</el-radio>

            <el-radio label="区域3">区域3</el-radio>

          </el-radio-group>

        </div>

      </div>

      

 <!-- 右侧内容区-->

      <div class="topology-main">

 <!-- 指标选择 -->

        <div class="metrics-selector">

          <el-form :inline="true" :model="topologyMetricsForm" class="demo-form-inline">

            <el-form-item label="指标选择">

              <el-select v-model="topologyMetricsForm.primaryMetric" placeholder="选择默认流量" style="width: 120px;">

                <el-option label="网络流量监控" value="request_rate" />

                <el-option label="服务端错误率" value="server_error" />

                <el-option label="分组聚合详情" value="response_time" />

              </el-select>

            </el-form-item>

            <el-form-item label="分组依据">

              <el-select v-model="topologyMetricsForm.groupBy" placeholder="auto_service" style="width: 120px;">

                <el-option label="auto_service" value="auto_service" />

                <el-option label="主机名" value="host" />

                <el-option label="应用名称" value="app" />

              </el-select>

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="applyTopologyMetrics">应用</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 服务调用关系图-->

        <div class="topology-graph">

          <div class="graph-container">

            <div class="graph-controls">

              <div class="graph-control-item">

                <span>Top50</span>

              </div>

              <div class="graph-control-item">

                <span>按指标排序(降序排列)</span>

              </div>

              <div class="graph-control-item">

                <el-button size="small" @click="zoomIn">放大</el-button>

              </div>

              <div class="graph-control-item">

                <el-button size="small" @click="zoomOut">缩小</el-button>

              </div>

              <div class="graph-control-item">

                <el-button size="small" @click="autoLayout">自动布局排列</el-button>

              </div>

              <div class="graph-control-item">

                <el-button size="small" @click="fitView">适应画布大小</el-button>

              </div>

            </div>

            <div class="graph-content" ref="graphContainer">

 <!-- 动态拓扑图 -->

              <div class="mock-topology">

 <!-- 动态节点-->

                <div 

                  v-for="node in topologyData.nodes" 

                  :key="node.id"

                  class="node" 

                  :id="node.id"

                  :style="{ top: node.y + 'px', left: node.x + 'px' }"

                  @dblclick="handleNodeDoubleClick(node.id)"

                >

                  <div class="node-content">

                    <div class="node-icon">{{ getNodeIcon(node.type) }}</div>

                    <div class="node-label">{{ node.label }}</div>

                  </div>

                </div>

                

 <!-- 服务间调用连线 -->

                <svg class="connections" width="100%" height="100%">

                  <line 

                    v-for="edge in topologyData.edges" 

                    :key="edge.source + '-' + edge.target"

                    :x1="getNodeX(edge.source)"

                    :y1="getNodeY(edge.source)"

                    :x2="getNodeX(edge.target)"

                    :y2="getNodeY(edge.target)"

                    :stroke="edge.isAbnormal ? '#f56c6c' : '#409eff'"

                    :stroke-width="Math.max(1, Math.min(5, edge.value / 20))"

                  />

                </svg>

              </div>

            </div>

          </div>

        </div>

      </div>

    </div>

    

 <!-- 抽屉-->

    <el-drawer

      v-model="topologyDrawerVisible"

      title="服务调用详情"

      direction="rtl"

      size="50%"

    >

      <div class="topology-drawer">

        <h3>服务调用详情</h3>

        <el-descriptions :column="1" border>

          <el-descriptions-item label="服务名称">{{ selectedNode.name }}</el-descriptions-item>

          <el-descriptions-item label="网络流量监控">{{ selectedNode.requestRate }}</el-descriptions-item>

          <el-descriptions-item label="错误率">{{ selectedNode.errorRate }}</el-descriptions-item>

          <el-descriptions-item label="分组聚合详情">{{ selectedNode.responseTime }}</el-descriptions-item>

          <el-descriptions-item label="QPS">{{ selectedNode.qps }}</el-descriptions-item>

          <el-descriptions-item label="CPU使用率">{{ selectedNode.cpuUsage }}</el-descriptions-item>

          <el-descriptions-item label="内存使用率">{{ selectedNode.memoryUsage }}</el-descriptions-item>

        </el-descriptions>

        <div class="mt-4">

          <h4>关键指标</h4>

          <div class="drawer-charts">

            <div class="drawer-chart">

              <h5>网络流量监控</h5>

              <div class="mock-chart drawer-chart-content">

                <div class="chart-bars">

                  <div v-for="i in 20" :key="i" class="chart-bar drawer-bar" :style="{ height: requestRateData[i-1] + '%' }"></div>

                </div>

              </div>

            </div>

            <div class="drawer-chart">

              <h5>分组聚合详情</h5>

              <div class="mock-chart drawer-chart-content">

                <div class="chart-bars">

                  <div v-for="i in 20" :key="i" class="chart-bar drawer-bar" :style="{ height: responseTimeData[i-1] + '%' }"></div>

                </div>

              </div>

            </div>

          </div>

        </div>

      </div>

    </el-drawer>

  </div>

</template>



<script setup lang="ts">

// 生成模拟数据库（仅在组件挂载时调用一次，避免图表跳动）
const generateMockData = (max: number, min: number, count: number = 30) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

import { ref, onMounted } from 'vue'

import { ArrowDown } from '@element-plus/icons-vue'

import { api } from '../utils/api'
import { ElMessage } from 'element-plus'



// 拓扑搜索表单

const topologyForm = ref({

  snapshot: '15m',

  search: ''

})



const topologyActionsForm = ref({})



// 应用服务选择

const selectedApps = ref(['frontend', 'productcatalogservice', 'currencyservice'])



// 区域查询

const regionQuery = ref('全部')



// 拓扑分析指标表单

const topologyMetricsForm = ref({

  primaryMetric: 'request_rate',

  groupBy: 'auto_service'

})



// 图表数据库流

const requestRateData = ref([])

const responseTimeData = ref([])



// 服务调用关系

const graphContainer = ref<HTMLElement>()



// 拓扑数据库流

const topologyData = ref({

  nodes: [],

  edges: []

})

const loading = ref(false)

const error = ref('')



// 右侧抽屉弹窗

const topologyDrawerVisible = ref(false)

const selectedNode = ref({

  name: '',

  requestRate: '',

  errorRate: '',

  responseTime: '',

  qps: '',

  cpuUsage: '',

  memoryUsage: ''

})



// 异步加载拓扑图数据库

const loadTopologyData = async () => {

  loading.value = true

  error.value = ''

  try {

    const response = await api.getTopology({

      timeRange: topologyForm.value.snapshot,

      serviceType: topologyMetricsForm.value.groupBy

    })

    if (response.data && response.data.nodes && response.data.edges) {

      topologyData.value = response.data

    }

  } catch (err) {

 // console.error('加载拓扑数据库失败:', err)

    error.value = '加载数据库失败，请稍后重试'

 // 使用模拟数据库作为 fallback

    topologyData.value = {

      nodes: [

        {

          id: 'loadgenerator',

          label: 'loadgenerator',

          type: 'service',

          ip: '10.0.0.1',

          traffic: '1.2 GB/s',

          latency: '1ms',

          alerts: 0,

          x: 200,

          y: 100

        },

        {

          id: 'frontend',

          label: 'frontend',

          type: 'service',

          ip: '10.0.0.2',

          traffic: '800 MB/s',

          latency: '2ms',

          alerts: 0,

          x: 300,

          y: 200

        },

        {

          id: 'productcatalogservice',

          label: 'productcatalogservice',

          type: 'service',

          ip: '10.0.0.3',

          traffic: '500 MB/s',

          latency: '1ms',

          alerts: 0,

          x: 200,

          y: 300

        },

        {

          id: 'currencyservice',

          label: 'currencyservice',

          type: 'service',

          ip: '10.0.0.4',

          traffic: '300 MB/s',

          latency: '1ms',

          alerts: 0,

          x: 300,

          y: 300

        },

        {

          id: 'cartservice',

          label: 'cartservice',

          type: 'service',

          ip: '10.0.0.5',

          traffic: '400 MB/s',

          latency: '2ms',

          alerts: 0,

          x: 400,

          y: 300

        }

      ],

      edges: [

        {

          source: 'loadgenerator',

          target: 'frontend',

          value: 100,

          isAbnormal: false

        },

        {

          source: 'frontend',

          target: 'productcatalogservice',

          value: 50,

          isAbnormal: false

        },

        {

          source: 'frontend',

          target: 'currencyservice',

          value: 30,

          isAbnormal: false

        },

        {

          source: 'frontend',

          target: 'cartservice',

          value: 40,

          isAbnormal: false

        }

      ]

    }

  } finally {

    loading.value = false

  }

}



// 搜索流量分析

const searchTopology = () => {

  loadTopologyData()

}



// 保存搜索条件

const saveSearch = () => {
  ElMessage.info('功能开发中...')
}



// 显示数据库字段
const showDatabaseFields = () => {
  ElMessage.info('功能开发中...')
}



// 切换策略显示

const switchNameDisplay = () => {
  ElMessage.info('功能开发中...')
}



// 刷新流量分析

const refreshTopology = () => {

  loadTopologyData()

}



// 导出流量分析

const exportTopology = () => {
  ElMessage.info('功能开发中...')
}



// 调用链追踪应用拓扑指标

const applyTopologyMetrics = () => {

  loadTopologyData()

}



// 放大

const zoomIn = () => {
  ElMessage.info('功能开发中...')
}



// 缩小

const zoomOut = () => {
  ElMessage.info('功能开发中...')
}



// 自动布局排列

const autoLayout = () => {
  ElMessage.info('功能开发中...')
}



// 适应画布大小

const fitView = () => {
  ElMessage.info('功能开发中...')
}



// 性能分析异常检测

const handleNodeDoubleClick = (nodeId: string) => {

 // 模拟调用链分析

  selectedNode.value = {

    name: nodeId,

    requestRate: '1.2K',

    errorRate: '0%',

    responseTime: '12.5 ms',

    qps: '5.6K',

    cpuUsage: '45.2%',

    memoryUsage: '68.7%'

  }

  topologyDrawerVisible.value = true

  }



// 获取节点图标

const getNodeIcon = (type: string) => {

  const icons: Record<string, string> = {

    'physical': '物理机',

    'pod': '容器',

    'service': '服务',

    'frontend': '前端',

    'productcatalogservice': '容器',

    'currencyservice': '数据库',

    'cartservice': '缓存',

    'recommendationservice': '推荐',

    'adservice': '广告',

    'shippingservice': '物流',

    'checkoutservice': '结账',

    'redis-cart': 'Redis',

    'emailservice': '邮件',

    'paymentservice': '支付',

    'loadgenerator': '负载'

  }

  return icons[type] || '容器'

}



// 获取节点X坐标

const getNodeX = (nodeId: string) => {

  const node = topologyData.value.nodes.find(n => n.id === nodeId)

  return node?.x || 100

}



// 获取节点Y坐标

const getNodeY = (nodeId: string) => {

  const node = topologyData.value.nodes.find(n => n.id === nodeId)

  return node?.y || 100

}



// 页面加载时初始化数据库

onMounted(() => {
  requestRateData.value = generateMockData(80, 20, 20)
  responseTimeData.value = generateMockData(90, 10, 20)

  loadTopologyData()

})

</script>



<style scoped>

.app-topology-content {

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



.topology-search {

  flex: 1;

}



.topology-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.topology-content {

  display: flex;

  gap: 20px;

}



.topology-sidebar {

  width: 250px;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.filter-section {

  margin-bottom: 20px;

}



.filter-section h3 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.topology-main {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.metrics-selector {

  margin-bottom: 20px;

}



.topology-graph {

  background-color: #f5f7fa;

  border-radius: 4px;

  padding: 15px;

  min-height: 500px;

}



.graph-container {

  width: 100%;

  height: 100%;

}



.graph-controls {

  display: flex;

  gap: 10px;

  margin-bottom: 15px;

  align-items: center;

}



.graph-control-item {

  display: flex;

  align-items: center;

}



.graph-content {

  width: 100%;

  height: 450px;

  position: relative;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  background-color: white;

}



/* 模拟网络拓扑图*/

.mock-topology {

  position: relative;

  width: 100%;

  height: 100%;

  overflow: hidden;

}



.node {

  position: absolute;

  cursor: pointer;

  transition: all 0.3s ease;

}



.node:hover {

  transform: scale(1.05);

}



.node-content {

  display: flex;

  flex-direction: column;

  align-items: center;

  padding: 10px;

  background-color: white;

  border: 1px solid #409eff;

  border-radius: 4px;

  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);

}



.node-icon {

  font-size: 24px;

  margin-bottom: 5px;

}



.node-label {

  font-size: 12px;

  font-weight: bold;

  color: #303133;

  text-align: center;

  max-width: 100px;

  overflow: hidden;

  text-overflow: ellipsis;

  white-space: nowrap;

}



.connections {

  position: absolute;

  top: 0;

  left: 0;

  z-index: 0;

  pointer-events: none;

}



/* 节点位置样式 */

#loadgenerator {

  top: 50px;

  left: 200px;

}



#frontend {

  top: 150px;

  left: 300px;

}



#productcatalogservice {

  top: 250px;

  left: 200px;

}



#currencyservice {

  top: 250px;

  left: 300px;

}



#cartservice {

  top: 250px;

  left: 400px;

}



#recommendationservice {

  top: 250px;

  left: 500px;

}



#adservice {

  top: 250px;

  left: 600px;

}



#shippingservice {

  top: 250px;

  left: 700px;

}



#checkoutservice {

  top: 250px;

  left: 800px;

}



#redis-cart {

  top: 350px;

  left: 300px;

}



#emailservice {

  top: 350px;

  left: 400px;

}



#paymentservice {

  top: 350px;

  left: 500px;

}



.topology-drawer {

  padding: 20px;

}



.topology-drawer h3 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.topology-drawer h4 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.topology-drawer h5 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 12px;

  font-weight: bold;

  color: #303133;

}



.drawer-charts {

  display: flex;

  gap: 20px;

}



.drawer-chart {

  flex: 1;

}



.drawer-chart-content {

  height: 150px;

}



.drawer-bar {

  background-color: #67c23a;

  border-radius: 2px 2px 0 0;

}



.mt-4 {

  margin-top: 16px;

}



@media (max-width: 1200px) {

  .topology-content {

    flex-direction: column;

  }

  

  .topology-sidebar {

    width: 100%;

  }

  

  .drawer-charts {

    flex-direction: column;

  }

}



/* 模拟图表样式 */

.mock-chart {

  position: relative;

  width: 100%;

  overflow: hidden;

}



.chart-bars {

  display: flex;

  align-items: flex-end;

  height: 80%;

  gap: 2px;

  padding: 0 10px;

}



.chart-bar {

  flex: 1;

  min-height: 2px;

  transition: height 0.3s ease;

}



.chart-x-axis {

  display: flex;

  justify-content: space-between;

  height: 20%;

  padding: 0 10px;

  margin-top: 5px;

}



.x-axis-label {

  font-size: 10px;

  color: #909399;

  text-align: center;

  flex: 1;

}

</style>