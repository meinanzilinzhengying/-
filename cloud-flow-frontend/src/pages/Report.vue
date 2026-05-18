<template>

  <div class="report">

 <!-- 报表策略 -->

    <el-card v-if="currentModule === 'strategy'" class="mb-4">

      <template #header>

        <div class="card-header">

          <h2>报表策略</h2>

        </div>

      </template>

      

 <!-- 最近5分钟 -->

      <div class="search-filter mb-4">

        <el-form :inline="true" :model="searchForm" class="demo-form-inline">

          <el-form-item label="策略/视图名称">

            <el-input v-model="searchForm.keyword" placeholder="策略/视图名称" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="handleSearch">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      

 <!-- 报表策略列表 -->

      <el-table :data="reportStrategies" style="width: 100%">

        <el-table-column prop="name" label="策略(报表数量)" min-width="150">

          <template #default="scope">

            <el-button  @click="viewReports(scope.row)">{{ scope.row.name }}(已生成{{ scope.row.reportCount }}份</el-button>

          </template>

        </el-table-column>

        <el-table-column prop="type" label="类型" width="100" />

        <el-table-column prop="cycle" label="周期" width="100" />

        <el-table-column prop="object" label="对象" width="100">

          <template #default="scope">

            <el-button  @click="viewView(scope.row)">{{ scope.row.object }}</el-button>

          </template>

        </el-table-column>

        <el-table-column prop="recipient" label="推送邮箱" min-width="150" />

        <el-table-column prop="createTime" label="创建时间" width="180" sortable />

        <el-table-column prop="updateTime" label="更新时间" width="180" sortable />

        <el-table-column prop="status" label="状态" width="100">

          <template #default="scope">

            <el-switch v-model="scope.row.status" @change="toggleStatus(scope.row)" />

          </template>

        </el-table-column>

        <el-table-column label="操作" width="100" fixed="right">

          <template #default="scope">

            <el-button size="small" @click="editStrategy(scope.row)">

              <el-icon><Edit /></el-icon>

            </el-button>

            <el-button size="small" type="danger" @click="deleteStrategy(scope.row)">

              <el-icon><Delete /></el-icon>

            </el-button>

          </template>

        </el-table-column>

      </el-table>

      

 <!-- 数据库表 -->

      <div class="pagination mt-4">

        <div class="pagination-info">

          共 {{ reportStrategyTotal }} 条

        </div>

        <el-pagination
          background
          layout="prev, pager, next, jumper"
          :total="reportStrategyTotal"
          :page-size="pagination.strategy.pageSize"
          :current-page="pagination.strategy.currentPage"
          @current-change="(page) => handleCurrentChange(page, 'strategy')"
        />

      </div>

    </el-card>

    

 <!-- 报表下载 -->

    <el-card v-if="currentModule === 'download'" class="mb-4">

      <template #header>

        <div class="card-header">

          <h2>报表下载</h2>

        </div>

      </template>

      

 <!-- 最近5分钟 -->

      <div class="search-filter mb-4">

        <el-form :inline="true" :model="searchForm" class="demo-form-inline">

          <el-form-item label="视图名称/调度策略">

            <el-input v-model="searchForm.keyword" placeholder="视图名称/调度策略" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="handleSearch">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      

 <!-- 报表下载列表 -->

      <el-table :data="reports" style="width: 100%">

        <el-table-column prop="viewName" label="视图名称" min-width="150">

          <template #default="scope">

            <el-button  @click="viewViewFromReport(scope.row)">{{ scope.row.viewName }}</el-button>

          </template>

        </el-table-column>

        <el-table-column prop="strategyName" label="调度策略" min-width="150" />

        <el-table-column prop="type" label="类型" width="100" />

        <el-table-column prop="cycle" label="周期" width="100" />

        <el-table-column prop="startTime" label="开始时间" width="180" sortable />

        <el-table-column prop="endTime" label="结束时间" width="180" sortable />

        <el-table-column prop="createTime" label="创建时间" width="180" sortable />

        <el-table-column label="操作" width="150" fixed="right">

          <template #default="scope">

            <el-button size="small" @click="downloadReport(scope.row)">

              <el-icon><Download /></el-icon> 下载

            </el-button>

            <el-button size="small" type="danger" @click="deleteReport(scope.row)">

              <el-icon><Delete /></el-icon> 删除

            </el-button>

          </template>

        </el-table-column>

      </el-table>

      

 <!-- 数据库表 -->

      <div class="pagination mt-4">

        <div class="pagination-info">

          共 {{ reportTotal }} 条

        </div>

        <el-pagination

          background

          layout="prev, pager, next, jumper"

          :total="reportTotal"

          :page-size="pagination.download.pageSize"

          :current-page="pagination.download.currentPage"

          @current-change="(page) => handleCurrentChange(page, 'download')"

        />

      </div>

    </el-card>

  </div>

</template>



<script setup lang="ts">

import { ref, computed, reactive } from 'vue'

import { useRoute } from 'vue-router'

import { Edit, Delete, Download } from '@element-plus/icons-vue'

import { ElMessage } from 'element-plus'



// 路由

const route = useRoute()



// 当前模块

const currentModule = computed(() => {

  return route.path.split('/').pop() || 'strategy'

})



// 分页数据 - 各模块独立管理
const pagination = reactive({
  strategy: {
    pageSize: 10,
    currentPage: 1
  },
  download: {
    pageSize: 10,
    currentPage: 1
  }
})



// 搜索表单

const searchForm = ref({

  keyword: ''

})



// 报表统计概览

const reportStrategies = ref([

  {

    id: '1',

    name: 'test_detail2',

    type: '日报',

    cycle: '每天',

    object: 'test',

    recipient: 'test@example.com',

    createTime: '2023-07-11 11:42:39',

    updateTime: '2023-08-19 12:21:59',

    status: false,

    reportCount: 8

  },

  {

    id: '2',

    name: 'xxx',

    type: '周报',

    cycle: '每周',

    object: 'test',

    recipient: 'test@example.com',

    createTime: '2023-08-09 15:27:43',

    updateTime: '2023-08-09 15:27:43',

    status: false,

    reportCount: 2

  },

  {

    id: '3',

    name: '2121',

    type: '日报',

    cycle: '每天',

    object: 'cl-test1',

    recipient: 'test@example.com',

    createTime: '2023-07-08 19:49:48',

    updateTime: '2023-07-08 19:49:48',

    status: true,

    reportCount: 14

  },

  {

    id: '4',

    name: 'test',

    type: '周报',

    cycle: '每周',

    object: 'demo1',

    recipient: 'test@example.com',

    createTime: '2023-06-25 14:44:44',

    updateTime: '2023-06-30 11:35:49',

    status: false,

    reportCount: 5

  },

  {

    id: '5',

    name: 'test1',

    type: '日报',

    cycle: '每天',

    object: 'test',

    recipient: 'test@example.com',

    createTime: '2023-05-18 18:43:36',

    updateTime: '2023-05-29 13:41:48',

    status: false,

    reportCount: 1

  },

  {

    id: '6',

    name: '服务器temp',

    type: '日报',

    cycle: '每天',

    object: '前端-服务器',

    recipient: 'test@example.com',

    createTime: '2023-05-09 17:16:15',

    updateTime: '2023-05-09 17:16:15',

    status: true,

    reportCount: 14

  },

  {

    id: '7',

    name: '222',

    type: '日报',

    cycle: '每天',

    object: 'xxx',

    recipient: 'test@example.com',

    createTime: '2022-12-08 16:12:26',

    updateTime: '2022-12-08 16:12:26',

    status: true,

    reportCount: 64

  }

])



const reportStrategyTotal = ref(8)



// 报表数据

const reports = ref([

  {

    id: '1',

    viewName: 'test',

    strategyName: 'test_detail2',

    type: '日报',

    cycle: '每天',

    startTime: '2023-08-18 00:00:00',

    endTime: '2023-08-19 00:00:00',

    createTime: '2023-08-19 00:00:00'

  },

  {

    id: '2',

    viewName: 'test',

    strategyName: 'test_detail2',

    type: '日报',

    cycle: '每天',

    startTime: '2023-08-17 00:00:00',

    endTime: '2023-08-18 00:00:00',

    createTime: '2023-08-18 00:00:00'

  }

])



const reportTotal = ref(10)



// 仅支持操作

const handleSearch = () => {
  ElMessage.info('功能开发中...')
}



// 分页操作

const handleCurrentChange = (page: number, module: string) => {
  pagination[module as keyof typeof pagination].currentPage = page
}



// 报表统计详情

const viewReports = (strategy: any) => {
  ElMessage.info('功能开发中...')
}



const viewView = (strategy: any) => {
  ElMessage.info('功能开发中...')
}



const toggleStatus = (strategy: any) => {
  ElMessage.info('功能开发中...')
}



const editStrategy = (strategy: any) => {
  ElMessage.info('功能开发中...')
}



const deleteStrategy = (strategy: any) => {
  ElMessage.info('功能开发中...')
}



// 报表下载操作

const viewViewFromReport = (report: any) => {
  ElMessage.info('功能开发中...')
}



const downloadReport = (report: any) => {
  ElMessage.info('功能开发中...')
}



const deleteReport = (report: any) => {
  ElMessage.info('功能开发中...')
}

</script>



<style scoped>

.report {

  padding: 20px;

}



.card-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.search-filter {

  margin-bottom: 20px;

}



.pagination {

  margin-top: 20px;

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.pagination-info {

  color: #909399;

  font-size: 14px;

}



.action-buttons {

  display: flex;

  gap: 10px;

  margin-bottom: 20px;

}

</style>