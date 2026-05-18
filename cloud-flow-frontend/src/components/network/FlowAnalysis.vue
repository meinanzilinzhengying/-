<template>

  <div class="flow-analysis-content">

 <!-- TCP重传详情-->

    <div class="flow-header">

      <div class="time-range-selector">

        <el-form :inline="true" :model="flowForm" class="demo-form-inline">

          <el-form-item label="系统日志源">

            <el-select v-model="flowForm.dataSource" placeholder="选择数据源" style="width: 150px;">

              <el-option label="网络设备" value="network" />

              <el-option label="应用调用" value="application" />

            </el-select>

          </el-form-item>

          <el-form-item label="搜索快照管理">

            <el-select v-model="flowForm.timeRange" placeholder="最近1小时" style="width: 120px;">

              <el-option label="最近5分钟" value="5m" />

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="flowForm.search" placeholder="搜索关键词" style="width: 200px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchFlows">

              <el-icon><Search /></el-icon> 搜索

            </el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="analysis-controls">

        <el-button @click="refreshFlows">

          <el-icon><Refresh /></el-icon> 刷新

        </el-button>

        <el-button @click="exportFlows">

          <el-icon><Download /></el-icon> 导出数据库

        </el-button>

      </div>

    </div>

    

 <!-- 指标选择 -->

    <div class="metrics-selector">

      <el-form :inline="true" :model="flowMetricsForm" class="demo-form-inline">

        <el-form-item label="指标选择">

          <el-select v-model="flowMetricsForm.primaryMetric" placeholder="选择默认流量" style="width: 120px;">

            <el-option label="流量" value="traffic" />

            <el-option label="延迟" value="latency" />

            <el-option label="丢包率" value="packetLoss" />

          </el-select>

        </el-form-item>

        <el-form-item label="分组依据">

          <el-select v-model="flowMetricsForm.groupBy" placeholder="全部" style="width: 100px;">

            <el-option label="全部" value="all" />

            <el-option label="协议" value="protocol" />

            <el-option label="服务" value="service" />

          </el-select>

        </el-form-item>

        <el-form-item>

          <el-button type="primary" @click="applyFlowMetrics">应用</el-button>

        </el-form-item>

      </el-form>

    </div>

    

 <!-- 瀑布流排序规则描述-->

    <div class="flow-chart-container">

      <el-card class="mb-4">

        <template #header>

          <div class="chart-header">

            <h3>异常分析</h3>

            <div class="chart-actions">

              <el-button size="small" @click="toggleFlowChartType">

                <el-icon><View /></el-icon> 切换图表类型

              </el-button>

            </div>

          </div>

        </template>

        <div class="chart-content">

          <div class="mock-chart flow-chart">

            <div class="chart-bars">

              <div v-for="i in 60" :key="i" class="chart-bar flow-bar" :style="{ height: flowChartData[i-1] + '%' }"></div>

            </div>

            <div class="chart-x-axis">

              <div v-for="i in 12" :key="i" class="x-axis-label">{{ 13 + i }}:00</div>

            </div>

          </div>

        </div>

      </el-card>

    </div>

    

 <!-- 流日志表格-->

    <div class="flow-list">

      <el-table :data="flows" style="width: 100%">

        <el-table-column prop="startTime" label="开始时间" width="180" />

        <el-table-column prop="endTime" label="结束时间" width="180" />

        <el-table-column prop="clientIp" label="平均响应时间P" width="120" />

        <el-table-column prop="serverIp" label="请求链路追踪IP" width="120" />

        <el-table-column prop="protocol" label="网络协议" width="80" />

        <el-table-column prop="clientPort" label="客户端端口" width="100" />

        <el-table-column prop="serverPort" label="服务器端端口" width="100" />

        <el-table-column prop="networkLocation" label="流量的网络位置" width="120" />

        <el-table-column prop="status" label="状态" width="80">

          <template #default="scope">

            <el-tag type="success" v-if="scope.row.status === '正常'">正常</el-tag>

            <el-tag type="warning" v-else-if="scope.row.status === '警告'">警告</el-tag>

            <el-tag type="danger" v-else-if="scope.row.status === '错误'">错误</el-tag>

          </template>

        </el-table-column>

        <el-table-column prop="sendBytes" label="发送字节数" width="100" />

        <el-table-column prop="receiveBytes" label="接收字节数" width="100" />

        <el-table-column prop="topLatency" label="TOP延迟(渭s)" width="100" />

        <el-table-column prop="actions" label="操作" width="80">

          <template #default="scope">

            <el-button size="small" @click="viewFlowDetails(scope.row)">

              <el-icon><View /></el-icon> 详情

            </el-button>

          </template>

        </el-table-column>

      </el-table>

      

 <!-- 数据库表 -->

      <div class="pagination mt-4">

        <div class="pagination-info">

          共 {{ flowTotal }}  条

        </div>

        <el-pagination

          background

          layout="prev, pager, next, jumper"

          :total="flowTotal"

          :page-size="flowPageSize"

          :current-page="flowCurrentPage"

          @current-change="handleFlowPageChange"

        />

      </div>

    </div>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive } from 'vue';

import { Search, Refresh, Download, View } from '@element-plus/icons-vue';



// 表单流量详情

const flowForm = reactive({

  dataSource: 'network',

  timeRange: '1h',

  search: ''

});



const flowMetricsForm = reactive({

  primaryMetric: 'traffic',

  groupBy: 'all'

});



// 数据库表详情

const flowCurrentPage = ref(1);

const flowPageSize = ref(10);

const flowTotal = ref(100);



// 模拟流量详情

const flowChartData = [50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50]



const flows = Array(10).fill(0).map((_, index) => ({

  startTime: new Date(Date.now() - index * 60000).toLocaleString(),

  endTime: new Date(Date.now() - (index - 1) * 60000).toLocaleString(),

  clientIp: `192.168.1.${index + 10}`,

  serverIp: `192.168.2.${index + 20}`,

  protocol: ['TCP', 'UDP', 'ICMP'][index % 3],

  clientPort: 10000 + index,

  serverPort: 80 + index,

  networkLocation: '网络位置信息',

  status: ['正常', '警告', '错误'][index % 3],

  sendBytes: 50000,

  receiveBytes: 50000,

  topLatency: 500

}));



// 新建规则

const searchFlows = () => {

  };



const refreshFlows = () => {

  };



const exportFlows = () => {

  };



const applyFlowMetrics = () => {

  };



const toggleFlowChartType = () => {

  };



const viewFlowDetails = (row: any) => {

  };



const handleFlowPageChange = (page: number) => {

  flowCurrentPage.value = page;

  };

</script>



<style scoped>

.flow-analysis-content {

  padding: 20px;

}



.flow-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.time-range-selector {

  flex: 1;

}



.analysis-controls {

  display: flex;

  align-items: center;

  gap: 10px;

}



.metrics-selector {

  margin-bottom: 30px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.flow-chart-container {

  margin-bottom: 30px;

}



.chart-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.chart-actions {

  display: flex;

  align-items: center;

}



.chart-content {

  height: 300px;

}



.mock-chart {

  height: 100%;

  display: flex;

  flex-direction: column;

}



.chart-bars {

  flex: 1;

  display: flex;

  align-items: flex-end;

  gap: 1px;

  padding: 0 10px;

}



.chart-bar {

  flex: 1;

  border-radius: 2px 2px 0 0;

}



.flow-bar {

  background-color: #409eff;

}



.chart-x-axis {

  display: flex;

  justify-content: space-around;

  margin-top: 10px;

  font-size: 12px;

  color: #909399;

}



.flow-list {

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.flow-list .el-table {

  margin-bottom: 20px;

}



.flow-list .el-table th {

  background-color: #f5f7fa;

}



.flow-list .el-table td {

  padding: 10px;

}



.flow-list .el-tag {

  margin: 0;

}



.pagination {

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.pagination-info {

  color: #909399;

  font-size: 14px;

}



.mt-4 {

  margin-top: 16px;

}

</style>