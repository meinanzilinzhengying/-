<template>

  <div class="path-analysis-content">

 <!-- TCP重传详情-->

    <div class="path-header">

      <div class="time-range-selector">

        <el-form :inline="true" :model="pathForm" class="demo-form-inline">

          <el-form-item label="流量监控">

            <el-select v-model="pathForm.dataSource" placeholder="选择数据源" style="width: 150px;">

              <el-option label="网络设备" value="network" />

              <el-option label="应用调用" value="application" />

            </el-select>

          </el-form-item>

          <el-form-item label="搜索快照管理">

            <el-select v-model="pathForm.timeRange" placeholder="搜索关键词" style="width: 120px;">

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

            <el-input v-model="pathForm.search" placeholder="搜索关键词" style="width: 200px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchPaths">

              <el-icon><Search /></el-icon> 搜索

            </el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="analysis-controls">

        <el-button @click="startPathAnalysis">

          <el-icon><DataAnalysis /></el-icon> 网络流量分析

        </el-button>

        <el-button @click="exportPathData">

          <el-icon><Download /></el-icon> 导出数据库

        </el-button>

      </div>

    </div>

    

 <!-- 指标选择 -->

    <div class="metrics-selector">

      <el-form :inline="true" :model="pathMetricsForm" class="demo-form-inline">

        <el-form-item label="指标选择">

          <el-select v-model="pathMetricsForm.primaryMetric" placeholder="选择默认流量" style="width: 120px;">

            <el-option label="吞吐量" value="throughput" />

            <el-option label="延迟" value="latency" />

            <el-option label="丢包率" value="packetLoss" />

          </el-select>

        </el-form-item>

        <el-form-item label="分组依据">

          <el-select v-model="pathMetricsForm.groupBy" placeholder="auto_service" style="width: 120px;">

            <el-option label="auto_service" value="auto_service" />

            <el-option label="主机名" value="host" />

            <el-option label="应用名称" value="app" />

          </el-select>

        </el-form-item>

        <el-form-item>

          <el-button type="primary" @click="applyPathMetrics">应用</el-button>

        </el-form-item>

      </el-form>

    </div>

    

 <!-- 图表数据库展示 -->

    <div class="charts-container">

      <el-row :gutter="20">

        <el-col :span="8">

          <el-card class="mb-4">

            <template #header>

              <div class="chart-header">

                <h3>吞吐量</h3>

              </div>

            </template>

            <div class="chart-content">

              <div class="mock-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar throughput-bar" :style="{ height: pathThroughputData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">{{ 14 + i }}:30</div>

                </div>

              </div>

            </div>

          </el-card>

        </el-col>

        <el-col :span="8">

          <el-card class="mb-4">

            <template #header>

              <div class="chart-header">

                <h3>TCP重传率分析</h3>

              </div>

            </template>

            <div class="chart-content">

              <div class="mock-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar retransmit-bar" :style="{ height: pathRetransmitData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">{{ 14 + i }}:30</div>

                </div>

              </div>

            </div>

          </el-card>

        </el-col>

        <el-col :span="8">

          <el-card class="mb-4">

            <template #header>

              <div class="chart-header">

                <h3>TCP连接完成率分析</h3>

              </div>

            </template>

            <div class="chart-content">

              <div class="mock-chart">

                <div class="chart-bars">

                  <div v-for="i in 30" :key="i" class="chart-bar connect-bar" :style="{ height: pathConnectData[i-1] + '%' }"></div>

                </div>

                <div class="chart-x-axis">

                  <div v-for="i in 6" :key="i" class="x-axis-label">{{ 14 + i }}:30</div>

                </div>

              </div>

            </div>

          </el-card>

        </el-col>

      </el-row>

    </div>

    

 <!-- 路径列表 -->

    <div class="path-list">

      <el-table :data="paths" style="width: 100%">

        <el-table-column prop="source" label="源地址" width="150" />

        <el-table-column prop="destination" label="目标" width="150" />

        <el-table-column prop="service" label="服务" width="120" />

        <el-table-column prop="totalBytes" label="总流量(字节)" width="120" />

        <el-table-column prop="sendRate" label="发送速率(字节/秒)" width="120" />

        <el-table-column prop="receiveRate" label="接收速率(字节/秒)" width="120" />

        <el-table-column prop="sendPacketRate" label="发送包速率(包/秒)" width="120" />

        <el-table-column prop="receivePacketRate" label="接收包速率(包/秒)" width="120" />

        <el-table-column prop="avgLatency" label="平均延迟(ms)" width="100" />

        <el-table-column prop="p95Latency" label="P95延迟(ms)" width="100" />

        <el-table-column prop="retransmitRatio" label="TCP重传率分析(%)" width="120" />

        <el-table-column prop="connectRatio" label="TCP连接完成率(%)" width="120" />

        <el-table-column prop="packetLossRatio" label="丢包率(%)" width="100" />

        <el-table-column prop="responseTime" label="平均延迟(ms)" width="120" />

        <el-table-column prop="pathLength" label="路径长度" width="80" />

      </el-table>

      

 <!-- 数据库表 -->

      <div class="pagination mt-4">

        <div class="pagination-info">

          共 {{ pathTotal }} 条

        </div>

        <el-pagination

          background

          layout="prev, pager, next, jumper"

          :total="pathTotal"

          :page-size="pathPageSize"

          :current-page="pathCurrentPage"

          @current-change="handlePathPageChange"

        />

      </div>

    </div>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive } from 'vue';

import { Search, DataAnalysis, Download } from '@element-plus/icons-vue';



// 表单流量详情

const pathForm = reactive({

  dataSource: 'network',

  timeRange: '5m',

  search: ''

});



const pathMetricsForm = reactive({

  primaryMetric: 'throughput',

  groupBy: 'auto_service'

});



// 数据库表详情

const pathCurrentPage = ref(1);

const pathPageSize = ref(10);

const pathTotal = ref(100);



// 模拟流量详情

const pathThroughputData = [50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50]

const pathRetransmitData = [25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25, 25]

const pathConnectData = [50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50]



const paths = Array(10).fill(0).map((_, index) => ({

  source: `192.168.1.${index + 10}`,

  destination: `192.168.2.${index + 20}`,

  service: `service-${index + 1}`,

  totalBytes: 500000,

  sendRate: 5000,

  receiveRate: 5000,

  sendPacketRate: 500,

  receivePacketRate: 500,

  avgLatency: 50,

  p95Latency: 100,

  retransmitRatio: 5,

  connectRatio: 50,

  packetLossRatio: 2.5,

  responseTime: 2.500,

  pathLength: 3

}));



// 新建规则

const searchPaths = () => {

  };



const startPathAnalysis = () => {

  };



const exportPathData = () => {

  };



const applyPathMetrics = () => {

  };



const handlePathPageChange = (page: number) => {

  pathCurrentPage.value = page;

  };

</script>



<style scoped>

.path-analysis-content {

  padding: 20px;

}



.path-header {

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



.charts-container {

  margin-bottom: 30px;

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

  gap: 2px;

  padding: 0 10px;

}



.chart-bar {

  flex: 1;

  border-radius: 2px 2px 0 0;

}



.throughput-bar {

  background-color: #409eff;

}



.retransmit-bar {

  background-color: #f56c6c;

}



.connect-bar {

  background-color: #67c23a;

}



.chart-x-axis {

  display: flex;

  justify-content: space-around;

  margin-top: 10px;

  font-size: 12px;

  color: #909399;

}



.path-list {

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.path-list .el-table {

  margin-bottom: 20px;

}



.path-list .el-table th {

  background-color: #f5f7fa;

}



.path-list .el-table td {

  padding: 10px;

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