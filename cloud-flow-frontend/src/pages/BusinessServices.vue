<template>

  <div class="business-services">

    <div class="services-header">

      <h3>服务列表</h3>

      <div class="header-actions">

        <el-form :inline="true" :model="searchForm" class="demo-form-inline">

          <el-form-item>

            <el-input v-model="searchForm.keyword" placeholder="搜索服务" style="width: 200px;" />

          </el-form-item>

          <el-form-item>

            <el-select v-model="searchForm.business" placeholder="选择业务" style="width: 150px;">

              <el-option label="全部" value="" />

              <el-option label="电商业务" value="电商业务" />

              <el-option label="物流业务" value="物流业务" />

              <el-option label="营销业务" value="营销业务" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchServices">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    <div class="services-content">

      <el-table :data="servicesData" style="width: 100%" @row-click="handleRowClick">

        <el-table-column prop="name" label="服务名称" width="180" />

        <el-table-column prop="business" label="所属业务线" width="150" />

        <el-table-column prop="status" label="状态" width="80">

          <template #default="scope">

            <el-tag :type="scope.row.status === '正常' ? 'success' : 'danger'">

              {{ scope.row.status }}

            </el-tag>

          </template>

        </el-table-column>

        <el-table-column prop="requestRate" label="网络流量监控" width="120" />

        <el-table-column prop="errorRate" label="错误率" width="100" />

        <el-table-column prop="responseTime" label="分组聚合详情" width="120" />

        <el-table-column prop="qps" label="QPS" width="80" />

        <el-table-column label="操作" width="120" fixed="right">

          <template #default="scope">

            <el-button size="small" @click="viewService(scope.row)">

              查看

            </el-button>

          </template>

        </el-table-column>

      </el-table>

      <div class="pagination mt-4">

        <div class="pagination-info">

          共 {{ total }} 条

        </div>

        <el-pagination

          background

          layout="prev, pager, next, jumper"

          :total="total"

          :page-size="pageSize"

          :current-page="currentPage"

          @current-change="handlePageChange"

        />

      </div>

    </div>

    

 <!-- 请求链路日志抽屉-->

    <el-drawer

      v-model="drawerVisible"

      title="服务调用详情"

      direction="rtl"

      size="50%"

    >

      <div class="service-drawer">

        <h4>{{ selectedService.name }} - 服务调用详情</h4>

        <el-descriptions :column="1" border>

          <el-descriptions-item label="服务名称">{{ selectedService.name }}</el-descriptions-item>

          <el-descriptions-item label="所属业务线">{{ selectedService.business }}</el-descriptions-item>

          <el-descriptions-item label="状态">{{ selectedService.status }}</el-descriptions-item>

          <el-descriptions-item label="网络流量监控">{{ selectedService.requestRate }}</el-descriptions-item>

          <el-descriptions-item label="错误率">{{ selectedService.errorRate }}</el-descriptions-item>

          <el-descriptions-item label="分组聚合详情">{{ selectedService.responseTime }}</el-descriptions-item>

          <el-descriptions-item label="QPS">{{ selectedService.qps }}</el-descriptions-item>

        </el-descriptions>

        <div class="mt-4">

          <h5>关键指标</h5>

          <div class="metrics-chart">

            <div class="mock-chart">

              <div class="chart-bars">

                <div v-for="i in 60" :key="i" class="chart-bar" :style="{ height: metricsData[i-1] + '%' }"></div>

              </div>

              <div class="chart-x-axis">

                <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

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



// 搜索处理函数

const searchForm = ref({

  keyword: '',

  business: ''

})



// 服务数据库流

const servicesData = ref([

  {

    id: 1,

    name: 'web-shop',

    business: '电商业务',

    status: '正常',

    requestRate: '3.74',

    errorRate: '0%',

    responseTime: '2.33 ms',

    qps: '1000'

  },

  {

    id: 2,

    name: 'svc-user',

    business: '电商业务',

    status: '正常',

    requestRate: '2.11',

    errorRate: '0%',

    responseTime: '1.2 ms',

    qps: '800'

  },

  {

    id: 3,

    name: 'svc-order',

    business: '电商业务',

    status: '正常',

    requestRate: '1.87',

    errorRate: '0%',

    responseTime: '1.5 ms',

    qps: '600'

  },

  {

    id: 4,

    name: 'svc-payment',

    business: '电商业务',

    status: '正常',

    requestRate: '1.5',

    errorRate: '0%',

    responseTime: '2.0 ms',

    qps: '500'

  },

  {

    id: 5,

    name: 'svc-shipping',

    business: '物流业务',

    status: '正常',

    requestRate: '1.2',

    errorRate: '0%',

    responseTime: '1.8 ms',

    qps: '400'

  },

  {

    id: 6,

    name: 'svc-warehouse',

    business: '物流业务',

    status: '正常',

    requestRate: '0.9',

    errorRate: '0%',

    responseTime: '1.3 ms',

    qps: '300'

  },

  {

    id: 7,

    name: 'svc-marketing',

    business: '营销业务',

    status: '正常',

    requestRate: '0.7',

    errorRate: '0%',

    responseTime: '1.0 ms',

    qps: '200'

  },

  {

    id: 8,

    name: 'svc-coupon',

    business: '营销业务',

    status: '正常',

    requestRate: '0.5',

    errorRate: '0%',

    responseTime: '0.8 ms',

    qps: '100'

  }

])



// 数据库表详情

const pageSize = ref(10)

const currentPage = ref(1)

const total = ref(8)



// 右侧抽屉弹窗

const drawerVisible = ref(false)

const selectedService = ref({

  name: '',

  business: '',

  status: '',

  requestRate: '',

  errorRate: '',

  responseTime: '',

  qps: ''

})



// 指标数据流

const metricsData = ref([])

onMounted(() => {
  metricsData.value = generateMockData(80, 20, 60)
})




// 搜索服务

const searchServices = () => {

 // 模拟搜索功能

}



// 处理行点击

const handleRowClick = (row: any) => {

  selectedService.value = row

  drawerVisible.value = true

  }



// 查看服务

const viewService = (row: any) => {

  selectedService.value = row

  drawerVisible.value = true

  }



// 数据库表变化

const handlePageChange = (page: number) => {

  currentPage.value = page

  }

</script>



<style scoped>

.business-services {

  padding: 20px;

}



.services-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

}



.services-header h3 {

  margin: 0;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.header-actions {

  display: flex;

  gap: 10px;

}



.services-content {

  background-color: #f5f7fa;

  border-radius: 4px;

  padding: 15px;

}



.services-content .el-table {

  margin-bottom: 20px;

}



.services-content .el-table th {

  background-color: #f5f7fa;

}



.services-content .el-table td {

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



.service-drawer {

  padding: 20px;

}



.service-drawer h4 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.service-drawer h5 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.metrics-chart {

  padding: 20px;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

}



/* 模拟图表样式 */

.mock-chart {

  position: relative;

  width: 100%;

  height: 200px;

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

  background-color: #409eff;

  border-radius: 2px 2px 0 0;

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



@media (max-width: 768px) {

  .services-header {

    flex-direction: column;

    align-items: flex-start;

    gap: 10px;

  }

  

  .pagination {

    flex-direction: column;

    align-items: flex-start;

    gap: 10px;

  }

}

</style>