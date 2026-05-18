<template>

  <div class="change-event-content">

 <!-- 变更事件 -->

    <div class="card-header">

      <h2>变更事件</h2>

      <div class="header-actions">

        <el-button size="small" @click="refreshEvents">

          <el-icon><Refresh /></el-icon> 刷新

        </el-button>

        <el-button size="small">

          <el-icon><Download /></el-icon> 导出数据库

        </el-button>

      </div>

    </div>

    

 <!-- 搜索和筛选-->

    <div class="search-filter mb-4">

      <div class="top-filters">

        <el-form :inline="true" :model="searchForm" class="demo-form-inline">

          <el-form-item label="数据库来源">

            <el-select v-model="searchForm.dataSource" placeholder="数据库来源">

              <el-option label="全部" value="all" />

              <el-option label="云服务器" value="cloud_server" />

              <el-option label="容器" value="container" />

              <el-option label="网络设备" value="network" />

            </el-select>

          </el-form-item>

          <el-form-item label="时间范围">

            <el-select v-model="searchForm.timeRange" placeholder="时间范围">

              <el-option label="最近5分钟" value="5m" />

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近24小时" value="24h" />

              <el-option label="自定义" value="custom" />

            </el-select>

          </el-form-item>

          <el-form-item v-if="searchForm.timeRange === 'custom'">

            <el-date-picker

              v-model="searchForm.dateRange"

              type="daterange"

              range-separator="至"

              start-placeholder="开始时间"

              end-placeholder="聚合分析详情"

              format="YYYY-MM-DD HH:mm:ss"

              value-format="YYYY-MM-DD HH:mm:ss"

            />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="handleSearch">搜索</el-button>

          </el-form-item>

        </el-form>

      </div>

      

      <div class="bottom-filters">

        <el-form :inline="true" :model="searchForm" class="demo-form-inline">

          <el-form-item label="事件类型">

            <el-select v-model="searchForm.eventType" placeholder="事件类型" multiple>

              <el-option label="资源创建事件" value="create" />

              <el-option label="资源变更" value="modify" />

              <el-option label="资源删除事件" value="delete" />

              <el-option label="配置变更事件" value="config_change" />

              <el-option label="状态变更事件" value="status_change" />

            </el-select>

          </el-form-item>

          <el-form-item label="用户">

            <el-input v-model="searchForm.user" placeholder="用户名" />

          </el-form-item>

          <el-form-item label="资源">

            <el-input v-model="searchForm.resource" placeholder="资源名称" />

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 事件瀑布图表 -->

    <div class="event-chart mb-4">

      <h3>事件瀑布</h3>

      <div class="chart-container" style="height: 200px; background-color: #f5f7fa; border-radius: 4px; display: flex; align-items: center; justify-content: center;">

        <div class="chart-placeholder">

          <el-icon class="icon-large"><DataAnalysis /></el-icon>

          <p>事件瀑布图表</p>

        </div>

      </div>

    </div>

    

 <!-- 事件类型筛选-->

    <div class="event-type-filter mb-4">

      <el-scrollbar>

        <div class="filter-tags">

          <el-tag :effect="searchForm.selectedTypes.includes('all') ? 'dark' : 'plain'" @click="toggleEventType('all')">

            全部

          </el-tag>

          <el-tag :effect="searchForm.selectedTypes.includes('create') ? 'dark' : 'plain'" @click="toggleEventType('create')" type="success">

            资源创建事件

          </el-tag>

          <el-tag :effect="searchForm.selectedTypes.includes('modify') ? 'dark' : 'plain'" @click="toggleEventType('modify')" type="warning">

            资源变更

          </el-tag>

          <el-tag :effect="searchForm.selectedTypes.includes('delete') ? 'dark' : 'plain'" @click="toggleEventType('delete')" type="danger">

            资源删除事件

          </el-tag>

          <el-tag :effect="searchForm.selectedTypes.includes('config_change') ? 'dark' : 'plain'" @click="toggleEventType('config_change')" type="info">

            配置变更事件

          </el-tag>

          <el-tag :effect="searchForm.selectedTypes.includes('status_change') ? 'dark' : 'plain'" @click="toggleEventType('status_change')" type="primary">

            状态变更事件

          </el-tag>

        </div>

      </el-scrollbar>

    </div>

    

 <!-- 变更事件列表 -->

    <el-table :data="filteredChangeEvents" style="width: 100%">

      <el-table-column prop="time" label="开始时间" width="180" />

      <el-table-column prop="cloudServer" label="云服务器" min-width="150" />

      <el-table-column prop="resource" label="资源" min-width="150" />

      <el-table-column prop="object" label="包含对象" min-width="120">

        <template #default="scope">

          <el-tag size="small">{{ scope.row.object }}</el-tag>

        </template>

      </el-table-column>

      <el-table-column prop="eventType" label="事件类型" width="120">

        <template #default="scope">

          <el-tag :type="getEventTypeType(scope.row.eventType)">{{ scope.row.eventType }}</el-tag>

        </template>

      </el-table-column>

      <el-table-column prop="eventInfo" label="事件信息" min-width="300" />

    </el-table>

    

 <!-- 数据库表 -->

    <div class="pagination mt-4">

      <el-pagination

        background

        layout="prev, pager, next, jumper"

        :total="filteredChangeEvents.length"

        :page-size="pageSize"

        :current-page="currentPage"

        @current-change="handleCurrentChange"

      />

    </div>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive, computed } from 'vue';

import { Refresh, Download, DataAnalysis } from '@element-plus/icons-vue';



// 表单数据

const searchForm = reactive({

  dataSource: 'all',

  timeRange: '5m',

  dateRange: null,

  eventType: [],

  user: '',

  resource: '',

  selectedTypes: ['all']

});



// 数据库表详情

const currentPage = ref(1);

const pageSize = ref(10);



// 模拟数据

const changeEvents = Array(50).fill(0).map((_, index) => ({

  time: new Date(Date.now() - index * 60000).toLocaleString(),

  cloudServer: `server-${index % 10 + 1}`,

  resource: `resource-${index % 15 + 1}`,

  object: ['instance', 'volume', 'network', 'security'][index % 4],

  eventType: ['资源创建事件', '资源变更', '资源删除事件', '配置变更事件', '状态变更事件'][index % 5],

  eventInfo: `事件详情 ${index + 1}`

}));



// 过滤后的事件

const filteredChangeEvents = computed(() => {

  return changeEvents;

});



// 刷新事件

const refreshEvents = () => {

  };



const handleSearch = () => {

  };



const toggleEventType = (type: string) => {

  const index = searchForm.selectedTypes.indexOf(type);

  if (index > -1) {

    searchForm.selectedTypes.splice(index, 1);

  } else {

    searchForm.selectedTypes.push(type);

  }

  };



const getEventTypeType = (eventType: string): string => {

  const typeMap: Record<string, string> = {

    '资源创建事件': 'success',

    '资源变更': 'warning',

    '资源删除事件': 'danger',

    '配置变更事件': 'info',

    '状态变更事件': 'primary'

  };

  return typeMap[eventType] || '';

};



const handleCurrentChange = (page: number) => {

  currentPage.value = page;

  };

</script>



<style scoped>

.change-event-content {

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



.header-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.search-filter {

  background-color: white;

  padding: 15px;

  border-radius: 4px;

  margin-bottom: 20px;

}



.top-filters {

  margin-bottom: 15px;

}



.event-chart {

  background-color: white;

  padding: 15px;

  border-radius: 4px;

  margin-bottom: 20px;

}



.event-chart h3 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

}



.chart-placeholder {

  text-align: center;

  color: #909399;

}



.icon-large {

  font-size: 48px;

  margin-bottom: 10px;

}



.event-type-filter {

  background-color: white;

  padding: 15px;

  border-radius: 4px;

  margin-bottom: 20px;

}



.filter-tags {

  display: flex;

  flex-wrap: wrap;

  gap: 10px;

}



.pagination {

  margin-top: 20px;

  display: flex;

  justify-content: flex-end;

}



.mt-4 {

  margin-top: 16px;

}

</style>