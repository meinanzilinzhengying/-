<template>

  <div class="service-topology">

 <!-- 顶部区域 -->

    <div class="top-section">

      <div class="top-tabs">

        <el-tabs v-model="activeTab" class="tab-container">

          <el-tab-pane label="全景图" name="overview" />

          <el-tab-pane label="列表" name="list" />

          <el-tab-pane label="拓扑" name="topology" />

        </el-tabs>

      </div>

      <div class="top-controls">

        <el-form :inline="true" :model="topForm" class="demo-form-inline">

          <el-form-item label="业务">

            <el-select v-model="topForm.business" placeholder="选择业务" style="width: 150px;">

              <el-option v-for="business in businessList" :key="business.id" :label="business.name" :value="business.id">

                <template #prefix>

                  <el-icon v-if="business.starred"><StarFilled /></el-icon>

                </template>

                {{ business.name }}

              </el-option>

            </el-select>

          </el-form-item>

          <el-form-item label="时间范围">

            <el-select v-model="topForm.timeRange" placeholder="选择时间范围" style="width: 120px;">

              <el-option label="最近5分钟" value="5m" />

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-button type="danger" @click="closePage">

              关闭

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 左侧导航和面包屑 -->

    <div class="left-section">

      <div class="left-nav">

        <el-menu

          :default-active="activeNav"

          class="left-menu"

          @select="handleNavSelect"

        >

          <el-menu-item index="services">

            <el-icon><Operation /></el-icon>

            <span>服务</span>

          </el-menu-item>

          <el-menu-item index="serviceGroups">

            <el-icon><Collection /></el-icon>

            <span>服务分组</span>

          </el-menu-item>

          <el-menu-item index="paths">

            <el-icon><Connection /></el-icon>

            <span>路径</span>

          </el-menu-item>

        </el-menu>

      </div>

      <div class="breadcrumb">

        <el-breadcrumb separator="/">

          <el-breadcrumb-item>电商业务</el-breadcrumb-item>

          <el-breadcrumb-item>服务分组</el-breadcrumb-item>

          <el-breadcrumb-item>web-shop</el-breadcrumb-item>

        </el-breadcrumb>

      </div>

    </div>

    

 <!-- 中心拓扑画布区域 -->

    <div class="center-section">

      <div class="topology-canvas" ref="topologyCanvas">

 <!-- 服务分组 -->

        <div class="service-group" style="top: 50px; left: 100px;">

          <div class="group-header" @click="toggleGroup(1)">

            <el-icon class="group-icon"><Collection /></el-icon>

            <span class="group-name">服务分组</span>

            <el-icon class="expand-icon"><component :is="group1Expanded ? ArrowUp : ArrowDown" /></el-icon>

          </div>

          <div class="group-content" v-if="group1Expanded">

            <div class="service" @click="openServiceDetail(service1)" style="top: 50px; left: 0;">

              <div class="service-header">

                <el-icon class="service-icon"><Operation /></el-icon>

                <span class="service-name">web-shop</span>

              </div>

              <div class="service-metrics">

                <div class="metric-item">

                  <div class="metric-label">分组聚合详情</div>

                  <div class="metric-bar error">

                    <div class="metric-progress" style="width: 70%;"></div>

                  </div>

                  <div class="metric-value error">120ms</div>

                </div>

                <div class="metric-item">

                  <div class="metric-label">错误率</div>

                  <div class="metric-bar">

                    <div class="metric-progress" style="width: 20%;"></div>

                  </div>

                  <div class="metric-value">0.5%</div>

                </div>

              </div>

            </div>

            <div class="service" @click="openServiceDetail(service2)" style="top: 150px; left: 0;">

              <div class="service-header">

                <el-icon class="service-icon"><Operation /></el-icon>

                <span class="service-name">svc-user</span>

              </div>

              <div class="service-metrics">

                <div class="metric-item">

                  <div class="metric-label">分组聚合详情</div>

                  <div class="metric-bar">

                    <div class="metric-progress" style="width: 40%;"></div>

                  </div>

                  <div class="metric-value">80ms</div>

                </div>

                <div class="metric-item">

                  <div class="metric-label">错误率</div>

                  <div class="metric-bar">

                    <div class="metric-progress" style="width: 10%;"></div>

                  </div>

                  <div class="metric-value">0.2%</div>

                </div>

              </div>

            </div>

          </div>

        </div>

        

 <!-- 服务分组 -->

        <div class="service-group" style="top: 50px; left: 400px;">

          <div class="group-header" @click="toggleGroup(2)">

            <el-icon class="group-icon"><Collection /></el-icon>

            <span class="group-name">服务分组</span>

            <el-icon class="expand-icon"><component :is="group2Expanded ? ArrowUp : ArrowDown" /></el-icon>

          </div>

          <div class="group-content" v-if="group2Expanded">

            <div class="service" @click="openServiceDetail(service3)" style="top: 50px; left: 0;">

              <div class="service-header">

                <el-icon class="service-icon"><Operation /></el-icon>

                <span class="service-name">svc-order</span>

              </div>

              <div class="service-metrics">

                <div class="metric-item">

                  <div class="metric-label">分组聚合详情</div>

                  <div class="metric-bar">

                    <div class="metric-progress" style="width: 45%;"></div>

                  </div>

                  <div class="metric-value">95ms</div>

                </div>

                <div class="metric-item">

                  <div class="metric-label">错误率</div>

                  <div class="metric-bar">

                    <div class="metric-progress" style="width: 40%;"></div>

                  </div>

                  <div class="metric-value">0.8%</div>

                </div>

              </div>

            </div>

            <div class="service" @click="openServiceDetail(service4)" style="top: 150px; left: 0;">

              <div class="service-header">

                <el-icon class="service-icon"><Operation /></el-icon>

                <span class="service-name">svc-payment</span>

              </div>

              <div class="service-metrics">

                <div class="metric-item">

                  <div class="metric-label">分组聚合详情</div>

                  <div class="metric-bar error">

                    <div class="metric-progress" style="width: 80%;"></div>

                  </div>

                  <div class="metric-value error">150ms</div>

                </div>

                <div class="metric-item">

                  <div class="metric-label">错误率</div>

                  <div class="metric-bar error">

                    <div class="metric-progress" style="width: 60%;"></div>

                  </div>

                  <div class="metric-value error">2.0%</div>

                </div>

              </div>

            </div>

          </div>

        </div>

        

 <!-- 服务分组 -->

        <div class="service-group" style="top: 300px; left: 250px;">

          <div class="group-header" @click="toggleGroup(3)">

            <el-icon class="group-icon"><Collection /></el-icon>

            <span class="group-name">服务分组</span>

            <el-icon class="expand-icon"><component :is="group3Expanded ? ArrowUp : ArrowDown" /></el-icon>

          </div>

          <div class="group-content" v-if="group3Expanded">

            <div class="service" @click="openServiceDetail(service5)" style="top: 50px; left: 0;">

              <div class="service-header">

                <el-icon class="service-icon"><Operation /></el-icon>

                <span class="service-name">svc-shipping</span>

              </div>

              <div class="service-metrics">

                <div class="metric-item">

                  <div class="metric-label">分组聚合详情</div>

                  <div class="metric-bar">

                    <div class="metric-progress" style="width: 35%;"></div>

                  </div>

                  <div class="metric-value">75ms</div>

                </div>

                <div class="metric-item">

                  <div class="metric-label">错误率</div>

                  <div class="metric-bar">

                    <div class="metric-progress" style="width: 5%;"></div>

                  </div>

                  <div class="metric-value">0.1%</div>

                </div>

              </div>

            </div>

          </div>

        </div>

        

 <!-- 路径 -->

        <svg class="topology-connections" width="100%" height="100%">

          <line x1="250" y1="100" x2="400" y2="100" stroke="#409eff" stroke-width="2" @click="openPathDetail(path1)" />

          <line x1="250" y1="200" x2="400" y2="150" stroke="#409eff" stroke-width="2" @click="openPathDetail(path2)" />

          <line x1="550" y1="100" x2="400" y2="350" stroke="#409eff" stroke-width="2" @click="openPathDetail(path3)" />

        </svg>

      </div>

      

 <!-- 右下角操作按钮 -->

      <div class="canvas-controls">

        <el-button @click="toggleLayoutMode" :class="{ 'active': layoutMode }">

          排布

        </el-button>

        <el-button @click="toggleEditMode" :class="{ 'active': editMode }">

          编辑

        </el-button>

        <el-button @click="showLegend">

          图例

        </el-button>

        <el-button @click="zoomIn">

          放大

        </el-button>

        <el-button @click="zoomOut">

          缩小

        </el-button>

        <el-button @click="toggleLock">

          {{ locked ? '解锁' : '锁定画布' }}

        </el-button>

      </div>

    </div>

    

 <!-- 右侧抽屉 -->

    <el-drawer

      v-model="drawerVisible"

      title="服务详情"

      direction="rtl"

      size="800px"

    >

      <div class="drawer-content">

 <!-- 上半：调用拓扑图 -->

        <div class="call-topology">

          <h4>调用拓扑图</h4>

          <div class="topology-group-selector">

            <el-select v-model="topologyGroup" placeholder="选择服务分组" style="width: 150px;">

              <el-option label="全部" value="all" />

              <el-option label="服务分组" value="group1" />

              <el-option label="服务分组" value="group2" />

              <el-option label="服务分组" value="group3" />

            </el-select>

          </div>

          <div class="call-topology-canvas">

            <div class="call-node">web-shop</div>

            <div class="call-arrow">→</div>

            <div class="call-node">svc-order</div>

            <div class="call-arrow">→</div>

            <div class="call-node">svc-payment</div>

          </div>

        </div>

        

 <!-- 下半：标签页 -->

        <div class="drawer-tabs">

          <el-tabs v-model="drawerActiveTab">

            <el-tab-pane label="知识图谱" name="knowledge">

              <div class="knowledge-graph">

                <h5>Tag/属性列表</h5>

                <el-table :data="knowledgeData" style="width: 100%">

                  <el-table-column prop="key" label="Key" width="150" />

                  <el-table-column prop="value" label="Value" />

                  <el-table-column prop="type" label="类型" width="100" />

                </el-table>

              </div>

            </el-tab-pane>

            <el-tab-pane label="应用性能" name="appPerformance">

              <div class="app-performance">

                <h5>应用性能概览</h5>

                <div class="metrics-cards">

                  <el-card class="metric-card">

                    <div class="metric-card-content">

                      <div class="metric-card-value">1000</div>

                      <div class="metric-card-label">吞吐</div>

                    </div>

                  </el-card>

                  <el-card class="metric-card">

                    <div class="metric-card-content">

                      <div class="metric-card-value">120ms</div>

                      <div class="metric-card-label">时延</div>

                    </div>

                  </el-card>

                  <el-card class="metric-card">

                    <div class="metric-card-content">

                      <div class="metric-card-value error">0.5%</div>

                      <div class="metric-card-label">错误率</div>

                    </div>

                  </el-card>

                </div>

                <h5 class="mt-4">端点列表</h5>

                <el-table :data="endpointData" style="width: 100%" @row-click="openEndpointDetail">

                  <el-table-column prop="name" label="端点名称" width="150" />

                  <el-table-column prop="ip" label="IP地址" width="150" />

                  <el-table-column prop="port" label="端口号" width="100" />

                  <el-table-column prop="status" label="状态" width="100">

                    <template #default="scope">

                      <el-tag :type="scope.row.status === '正常' ? 'success' : 'danger'">

                        {{ scope.row.status }}

                      </el-tag>

                    </template>

                  </el-table-column>

                </el-table>

              </div>

            </el-tab-pane>

            <el-tab-pane label="网络性能" name="netPerformance">

              <div class="net-performance">

                <h5>网络性能概览</h5>

                <div class="metrics-cards">

                  <el-card class="metric-card">

                    <div class="metric-card-content">

                      <div class="metric-card-value">1.2MB/s</div>

                      <div class="metric-card-label">吞吐</div>

                    </div>

                  </el-card>

                  <el-card class="metric-card">

                    <div class="metric-card-content">

                      <div class="metric-card-value">5ms</div>

                      <div class="metric-card-label">时延</div>

                    </div>

                  </el-card>

                  <el-card class="metric-card">

                    <div class="metric-card-content">

                      <div class="metric-card-value">0.1%</div>

                      <div class="metric-card-label">错误率</div>

                    </div>

                  </el-card>

                </div>

                <h5 class="mt-4">端口号列表</h5>

                <el-table :data="portData" style="width: 100%" @row-click="openPortDetail">

                  <el-table-column prop="port" label="端口号" width="100" />

                  <el-table-column prop="protocol" label="协议" width="100" />

                  <el-table-column prop="traffic" label="流量" width="150" />

                  <el-table-column prop="status" label="状态" width="100">

                    <template #default="scope">

                      <el-tag :type="scope.row.status === '正常' ? 'success' : 'danger'">

                        {{ scope.row.status }}

                      </el-tag>

                    </template>

                  </el-table-column>

                </el-table>

              </div>

            </el-tab-pane>

            <el-tab-pane label="基础设施" name="infrastructure">

              <div class="infrastructure">

                <h5>主机信息</h5>

                <el-table :data="hostData" style="width: 100%">

                  <el-table-column prop="name" label="主机名称" width="150" />

                  <el-table-column prop="ip" label="IP地址" width="150" />

                  <el-table-column prop="cpu" label="CPU使用率" width="120" />

                  <el-table-column prop="mem" label="内存使用率" width="120" />

                  <el-table-column prop="status" label="状态" width="100">

                    <template #default="scope">

                      <el-tag :type="scope.row.status === '正常' ? 'success' : 'danger'">

                        {{ scope.row.status }}

                      </el-tag>

                    </template>

                  </el-table-column>

                </el-table>

              </div>

            </el-tab-pane>

            <el-tab-pane label="事件" name="events">

              <div class="events">

                <h5>资源变更事件</h5>

                <el-table :data="eventData" style="width: 100%">

                  <el-table-column prop="time" label="时间范围" width="180" />

                  <el-table-column prop="type" label="事件类型" width="120" />

                  <el-table-column prop="resource" label="资源" width="150" />

                  <el-table-column prop="message" label="事件信息" />

                </el-table>

              </div>

            </el-tab-pane>

          </el-tabs>

        </div>

      </div>

    </el-drawer>

    

 <!-- 下级抽屉 -->

    <el-drawer

      v-model="subDrawerVisible"

      title="端点详情"

      direction="rtl"

      size="600px"

    >

      <div class="sub-drawer-content">

        <h4>RED指标</h4>

        <div class="red-metrics">

          <el-card class="red-metric-card">

            <div class="red-metric-content">

              <div class="red-metric-value">1000</div>

              <div class="red-metric-label">Rate (QPS)</div>

            </div>

          </el-card>

          <el-card class="red-metric-card">

            <div class="red-metric-content">

              <div class="red-metric-value">120ms</div>

              <div class="red-metric-label">Error (错误率</div>

            </div>

          </el-card>

          <el-card class="red-metric-card">

            <div class="red-metric-content">

              <div class="red-metric-value">0.5%</div>

              <div class="red-metric-label">Duration (分组聚合详情)</div>

            </div>

          </el-card>

        </div>

        <h4 class="mt-4">详细日志</h4>

        <el-table :data="logData" style="width: 100%">

          <el-table-column prop="time" label="时间范围" width="180" />

          <el-table-column prop="level" label="级别" width="100" />

          <el-table-column prop="message" label="日志内容" />

        </el-table>

        <h4 class="mt-4">异常分析</h4>

        <div class="anomaly-analysis">

          <p>最近30分钟内检测到 2 次异常，主要集中在响应时间超标</p>

          <el-button type="primary" @click="analyzeAnomaly">

            分析错误率

          </el-button>

        </div>

      </div>

    </el-drawer>

  </div>

</template>



<script setup lang="ts">

import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { Operation, Collection, Connection, StarFilled, ArrowUp, ArrowDown } from '@element-plus/icons-vue'


// 路由实例
const router = useRouter()

// 刷新标签

const activeTab = ref('topology')



// 顶部表单

const topForm = ref({

  business: 1,

  timeRange: '5m'

})



// 业务列表

const businessList = ref([

  { id: 1, name: '电商业务', starred: true },

  { id: 2, name: '物流业务', starred: false },

  { id: 3, name: '营销业务', starred: false }

])



// 左侧导航

const activeNav = ref('services')



// 服务组展开开关

const group1Expanded = ref(true)

const group2Expanded = ref(true)

const group3Expanded = ref(true)



// 模式开关

const layoutMode = ref(false)

const editMode = ref(false)

const locked = ref(false)



// 服务数据库

const service1 = ref({

  id: 1,

  name: 'web-shop',

  serviceGroup: '服务分组',

  responseTime: 120,

  errorRate: 0.5,

  qps: 1000,

  requestRate: 1.2

})



const service2 = ref({

  id: 2,

  name: 'svc-user',

  serviceGroup: '服务分组',

  responseTime: 80,

  errorRate: 0.2,

  qps: 800,

  requestRate: 0.9

})



const service3 = ref({

  id: 3,

  name: 'svc-order',

  serviceGroup: '服务分组',

  responseTime: 95,

  errorRate: 0.8,

  qps: 600,

  requestRate: 0.7

})



const service4 = ref({

  id: 4,

  name: 'svc-payment',

  serviceGroup: '服务分组',

  responseTime: 150,

  errorRate: 2.0,

  qps: 500,

  requestRate: 0.6

})



const service5 = ref({

  id: 5,

  name: 'svc-shipping',

  serviceGroup: '服务分组',

  responseTime: 75,

  errorRate: 0.1,

  qps: 400,

  requestRate: 0.5

})



// 路径数据库

const path1 = ref({

  id: 1,

  source: 'web-shop',

  target: 'svc-order',

  requestRate: 1.87,

  errorRate: 0,

  responseTime: 1.5

})



const path2 = ref({

  id: 2,

  source: 'svc-user',

  target: 'svc-order',

  requestRate: 1.2,

  errorRate: 0,

  responseTime: 1.0

})



const path3 = ref({

  id: 3,

  source: 'svc-order',

  target: 'svc-shipping',

  requestRate: 0.9,

  errorRate: 0,

  responseTime: 1.2

})



// 抽屉

const drawerVisible = ref(false)

const drawerActiveTab = ref('knowledge')

const topologyGroup = ref('all')



// 下级抽屉

const subDrawerVisible = ref(false)



// 知识图谱数据库

const knowledgeData = ref([

  { key: 'service_name', value: 'web-shop', type: 'Tag' },

  { key: 'service_group', value: '服务分组', type: 'Tag' },

  { key: 'region', value: '区域1', type: 'Tag' },

  { key: 'ip', value: '192.168.1.100', type: '属性' },

  { key: 'port', value: '8080', type: '属性' }

])



// 端点数据库

const endpointData = ref([

  { id: 1, name: '端点1', ip: '192.168.1.100', port: '8080', status: '正常' },

  { id: 2, name: '端点2', ip: '192.168.1.101', port: '8081', status: '错误率' },

  { id: 3, name: '端点3', ip: '192.168.1.102', port: '8082', status: '正常' }

])



// 端口号数据库

const portData = ref([

  { id: 1, port: '8080', protocol: 'HTTP', traffic: '1.2MB/s', status: '正常' },

  { id: 2, port: '8081', protocol: 'HTTPS', traffic: '800KB/s', status: '正常' },

  { id: 3, port: '3306', protocol: 'MySQL', traffic: '500KB/s', status: '正常' }

])



// 主机数据库

const hostData = ref([

  { id: 1, name: '主机1', ip: '192.168.1.1', cpu: '40%', mem: '60%', status: '正常' },

  { id: 2, name: '主机2', ip: '192.168.1.2', cpu: '70%', mem: '80%', status: '错误率' },

  { id: 3, name: '主机3', ip: '192.168.1.3', cpu: '30%', mem: '50%', status: '正常' }

])



// 事件数据库

const eventData = ref([

  { id: 1, time: '2023-09-01 10:00:00', type: '服务告警', resource: 'web-shop', message: '服务告警：web-shop服务' },

  { id: 2, time: '2023-09-01 11:00:00', type: '配置变更', resource: 'svc-order', message: '配置变更：svc-order服务配置' },

  { id: 3, time: '2023-09-01 12:00:00', type: '删除网络', resource: 'old-service', message: '删除old-service服务' }

])



// 日志数据库

const logData = ref([

  { id: 1, time: '2023-09-01 10:00:00', level: 'INFO', message: '请求处理成功' },

  { id: 2, time: '2023-09-01 10:05:00', level: 'ERROR', message: '响应时间超标: 120ms' },

  { id: 3, time: '2023-09-01 10:10:00', level: 'INFO', message: '请求处理成功' }

])



// 处理导航选择

const handleNavSelect = (key: string) => {

  activeNav.value = key

  }



// 切换服务组展开开关

const toggleGroup = (group: number) => {

  switch (group) {

    case 1:

      group1Expanded.value = !group1Expanded.value

      break

    case 2:

      group2Expanded.value = !group2Expanded.value

      break

    case 3:

      group3Expanded.value = !group3Expanded.value

      break

  }

}



// 打开服务详情

const openServiceDetail = (service: any) => {

  drawerVisible.value = true

  }



// 打开路径详情

const openPathDetail = (path: any) => {

  drawerVisible.value = true

  }



// 打开端点详情

const openEndpointDetail = (endpoint: any) => {

  subDrawerVisible.value = true

  }



// 打开端口详情

const openPortDetail = (port: any) => {

  subDrawerVisible.value = true

  }



// 切换布局模式

const toggleLayoutMode = () => {

  layoutMode.value = !layoutMode.value

  }



// 编辑模式切换

const toggleEditMode = () => {

  editMode.value = !editMode.value

  }



// 显示图例

const showLegend = () => {
  ElMessage.info('功能开发中...')
}



// 放大

const zoomIn = () => {
  ElMessage.info('功能开发中...')
}



// 缩小

const zoomOut = () => {
  ElMessage.info('功能开发中...')
}



// 解锁/锁定画布开关

const toggleLock = () => {

  locked.value = !locked.value

  }



// 关闭当前页面，返回服务列表
const closePage = () => {
  router.push('/service')
}



// 分析错误率

const analyzeAnomaly = () => {
  ElMessage.info('功能开发中...')
}

</script>



<style scoped>

.service-topology {

  padding: 24px;

  height: 100vh;

  display: flex;

  flex-direction: column;

  gap: 24px;

  background-color: #f5f7fa;

}



.top-section {

  display: flex;

  justify-content: space-between;

  align-items: center;

  background-color: white;

  border-radius: 4px;

  padding: 16px 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.tab-container {

  flex: 1;

}



.top-controls {

  display: flex;

  align-items: center;

  gap: 10px;

}



.left-section {

  display: flex;

  flex-direction: column;

  width: 200px;

  gap: 24px;

}



.left-nav {

  background-color: white;

  border-radius: 4px;

  padding: 16px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.left-menu {

  border-right: none;

}



.breadcrumb {

  background-color: white;

  border-radius: 4px;

  padding: 16px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.center-section {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  position: relative;

  overflow: hidden;

}



.topology-canvas {

  position: relative;

  width: 100%;

  height: calc(100% - 80px);

  border: 1px solid #e4e7ed;

  border-radius: 4px;

}



.service-group {

  position: absolute;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  background-color: #f9f9f9;

  padding: 10px;

  min-width: 200px;

}



.group-header {

  display: flex;

  align-items: center;

  gap: 8px;

  padding: 8px;

  cursor: pointer;

  border-bottom: 1px solid #e4e7ed;

}



.group-icon {

  color: #67c23a;

}



.group-name {

  flex: 1;

  font-weight: bold;

  color: #303133;

}



.expand-icon {

  color: #909399;

}



.group-content {

  position: relative;

  padding: 10px 0;

}



.service {

  position: relative;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  background-color: white;

  padding: 10px;

  margin-bottom: 10px;

  cursor: pointer;

  transition: all 0.3s ease;

}



.service:hover {

  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);

}



.service-header {

  display: flex;

  align-items: center;

  gap: 8px;

  margin-bottom: 10px;

}



.service-icon {

  color: #1677FF;

}



.service-name {

  font-weight: bold;

  color: #303133;

}



.service-metrics {

  display: flex;

  flex-direction: column;

  gap: 8px;

}



.metric-item {

  display: flex;

  align-items: center;

  gap: 8px;

  font-size: 12px;

}



.metric-label {

  width: 60px;

  color: #606266;

}



.metric-bar {

  flex: 1;

  height: 6px;

  background-color: #f0f0f0;

  border-radius: 3px;

  overflow: hidden;

  position: relative;

}



.metric-bar.error {

  background-color: #fff1f0;

}



.metric-progress {

  height: 100%;

  background-color: #1677FF;

  border-radius: 3px;

  transition: width 0.3s ease;

}



.metric-bar.error .metric-progress {

  background-color: #FF4D4F;

}



.metric-value {

  width: 60px;

  text-align: right;

  color: #606266;

}



.metric-value.error {

  color: #FF4D4F;

  font-weight: bold;

}



.topology-connections {

  position: absolute;

  top: 0;

  left: 0;

  z-index: 0;

  pointer-events: all;

}



.canvas-controls {

  position: absolute;

  bottom: 24px;

  right: 24px;

  display: flex;

  gap: 10px;

  background-color: white;

  padding: 10px;

  border-radius: 4px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.canvas-controls .el-button.active {

  background-color: #1677FF;

  color: white;

  border-color: #1677FF;

}



.drawer-content {

  padding: 24px;

  height: 100%;

  display: flex;

  flex-direction: column;

  gap: 24px;

}



.call-topology {

  background-color: #f5f7fa;

  border-radius: 4px;

  padding: 24px;

}



.call-topology h4 {

  margin-top: 0;

  margin-bottom: 16px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.topology-group-selector {

  margin-bottom: 24px;

}



.call-topology-canvas {

  display: flex;

  align-items: center;

  gap: 20px;

  padding: 20px;

  background-color: white;

  border-radius: 4px;

}



.call-node {

  padding: 10px 20px;

  background-color: #1677FF;

  color: white;

  border-radius: 4px;

  font-weight: bold;

}



.call-arrow {

  font-size: 20px;

  color: #1677FF;

  font-weight: bold;

}



.drawer-tabs {

  flex: 1;

  overflow: auto;

}



.knowledge-graph h5,

.app-performance h5,

.net-performance h5,

.infrastructure h5,

.events h5 {

  margin-top: 0;

  margin-bottom: 16px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.metrics-cards {

  display: flex;

  gap: 16px;

  margin-bottom: 24px;

}



.metric-card {

  flex: 1;

  text-align: center;

}



.metric-card-content {

  padding: 20px;

}



.metric-card-value {

  font-size: 24px;

  font-weight: bold;

  color: #303133;

  margin-bottom: 8px;

}



.metric-card-value.error {

  color: #FF4D4F;

}



.metric-card-label {

  font-size: 14px;

  color: #606266;

}



.mt-4 {

  margin-top: 24px;

}



.sub-drawer-content {

  padding: 24px;

  height: 100%;

  display: flex;

  flex-direction: column;

  gap: 24px;

}



.sub-drawer-content h4 {

  margin-top: 0;

  margin-bottom: 16px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.red-metrics {

  display: flex;

  gap: 16px;

  margin-bottom: 24px;

}



.red-metric-card {

  flex: 1;

  text-align: center;

}



.red-metric-content {

  padding: 20px;

}



.red-metric-value {

  font-size: 24px;

  font-weight: bold;

  color: #303133;

  margin-bottom: 8px;

}



.red-metric-label {

  font-size: 14px;

  color: #606266;

}



.anomaly-analysis {

  background-color: #fff1f0;

  border: 1px solid #ffccc7;

  border-radius: 4px;

  padding: 16px;

}



.anomaly-analysis p {

  margin-top: 0;

  margin-bottom: 16px;

  color: #cf1322;

}



@media (max-width: 1200px) {

  .top-section {

    flex-direction: column;

    align-items: flex-start;

    gap: 16px;

  }

  

  .left-section {

    width: 100%;

    flex-direction: row;

  }

  

  .left-nav {

    flex: 1;

  }

  

  .breadcrumb {

    flex: 2;

  }

  

  .metrics-cards {

    flex-direction: column;

  }

  

  .red-metrics {

    flex-direction: column;

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



:deep(.el-tabs__active-bar) {

  background-color: #1677FF;

}



:deep(.el-tabs__item.is-active) {

  color: #1677FF;

}

</style>