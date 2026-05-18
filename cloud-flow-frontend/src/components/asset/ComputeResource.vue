<template>

  <div class="compute-resource-content">

 <!-- 计算资源 -->

    <div class="card-header">

      <h2>计算资源</h2>

    </div>

    

 <!-- 标签页-->

    <el-tabs v-model="activeComputeTab">

 <!-- 云服务器标签页-->

      <el-tab-pane label="云服务器" name="cloud-server">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button type="primary" @click="createCloudServer">新建连接云服务器</el-button>

          <el-button @click="exportCloudServerCSV">导出数据库 CSV</el-button>

          <el-button @click="viewAllNics">查看全部网卡</el-button>

          <el-button @click="refreshCache">

            <el-icon><Refresh /></el-icon> 刷新采集器状态

          </el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="computeSearchForm" class="demo-form-inline">

            <el-form-item label="云服务器列表（最多显示32条）">

              <el-select v-model="computeSearchForm.filter" placeholder="全部">

                <el-option label="全部" value="all" />

                <el-option label="运行中" value="running" />

                <el-option label="已停止" value="stopped" />

                <el-option label="连接异常" value="error" />

              </el-select>

            </el-form-item>

            <el-form-item label="搜索">

              <el-input v-model="computeSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleComputeSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 云服务器列表 -->

        <el-table :data="cloudServers" style="width: 100%">

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="id" label="ID" width="150" />

          <el-table-column prop="vpc" label="VPC" width="120">

            <template #default="scope">

              <el-button  @click="viewVpc(scope.row)">{{ scope.row.vpc }}</el-button>

            </template>

          </el-table-column>

          <el-table-column prop="subnet" label="子网" width="120" />

          <el-table-column prop="internalIp" label="内网IP" width="150" />

          <el-table-column prop="externalIp" label="外网IP" width="150" />

          <el-table-column prop="collectorStatus" label="采集器状态(自动修复)" width="120">

            <template #default="scope">

              <el-tag :type="getStatusType(scope.row.collectorStatus)">{{ scope.row.collectorStatus }}</el-tag>

            </template>

          </el-table-column>

          <el-table-column prop="host" label="主机名称" width="120" />

          <el-table-column prop="source" label="来源" width="100" />

          <el-table-column prop="type" label="类型" width="120" />

          <el-table-column prop="cloudPlatform" label="云平台/可用区" width="150" />

          <el-table-column prop="cloudTag" label="cloud tag" width="120" />

          <el-table-column prop="createTime" label="创建时间" width="150" />

          <el-table-column prop="discoverTime" label="发现时间" width="150" />

          <el-table-column label="操作" width="150" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="editCloudServer(scope.row)">

                <el-icon><Edit /></el-icon>

              </el-button>

              <el-button size="small" @click="viewNics(scope.row)">

                <el-icon><Connection /></el-icon>

              </el-button>

              <el-button size="small" @click="viewCollectorStatus(scope.row)">

                <el-icon><Monitor /></el-icon>

              </el-button>

            </template>

          </el-table-column>

        </el-table>

        

 <!-- 数据库表 -->

        <div class="pagination mt-4">

          <el-pagination

            background

            layout="prev, pager, next, jumper"

            :total="cloudServerTotal"

            :page-size="pageSize"

            :current-page="currentPage"

            @current-change="handleCurrentChange"

          />

        </div>

      </el-tab-pane>

      

 <!-- 主机名称标签页 -->

      <el-tab-pane label="主机名称" name="host">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button type="primary" @click="createHost">新建主机名称</el-button>

          <el-button @click="exportHostCSV">导出数据库 CSV</el-button>

          <el-button @click="refreshCache">

            <el-icon><Refresh /></el-icon> 刷新采集器状态

          </el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="computeSearchForm" class="demo-form-inline">

            <el-form-item label="主机列表">

              <el-input v-model="computeSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleComputeSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 主机列表-->

        <el-table :data="hosts" style="width: 100%">

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="id" label="ID" width="150" />

          <el-table-column prop="region" label="区域" width="120" />

          <el-table-column prop="zone" label="可用区" width="120" />

          <el-table-column prop="internalIp" label="内网IP" width="150" />

          <el-table-column prop="externalIp" label="外网IP" width="150" />

          <el-table-column prop="cpu" label="CPU" width="100" />

          <el-table-column prop="memory" label="内存" width="100" />

          <el-table-column prop="nicCount" label="网卡数量/类型" width="100" />

          <el-table-column prop="collectorStatus" label="采集器状态(自动修复)" width="120">

            <template #default="scope">

              <el-tag :type="getStatusType(scope.row.collectorStatus)">{{ scope.row.collectorStatus }}</el-tag>

            </template>

          </el-table-column>

          <el-table-column prop="createTime" label="创建时间" width="150" />

          <el-table-column prop="discoverTime" label="发现时间" width="150" />

          <el-table-column label="操作" width="150" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="viewHost(scope.row)">

                <el-icon><View /></el-icon>

              </el-button>

              <el-button size="small" @click="editHost(scope.row)">

                <el-icon><Edit /></el-icon>

              </el-button>

            </template>

          </el-table-column>

        </el-table>

        

 <!-- 数据库表 -->

        <div class="pagination mt-4">

          <el-pagination

            background

            layout="prev, pager, next, jumper"

            :total="hostTotal"

            :page-size="pageSize"

            :current-page="currentPage"

            @current-change="handleCurrentChange"

          />

        </div>

      </el-tab-pane>

    </el-tabs>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive } from 'vue';

import { Refresh, Edit, Connection, Monitor, View } from '@element-plus/icons-vue';



// 标签页

const activeComputeTab = ref('cloud-server');



// 表单流量详情

const computeSearchForm = reactive({

  filter: 'all',

  keyword: ''

});



// 数据库表详情

const currentPage = ref(1);

const pageSize = ref(10);



// 模拟流量详情

const cloudServers = Array(32).fill(0).map((_, index) => ({

  name: `server-${index + 1}`,

  id: `cs-${index + 1}`,

  vpc: `vpc-${(index % 5) + 1}`,

  subnet: `subnet-${(index % 10) + 1}`,

  internalIp: `10.0.${Math.floor(index / 256)}.${index % 256}`,

  externalIp: index % 2 === 0 ? `203.0.${Math.floor(index / 256)}.${index % 256}` : '',

  collectorStatus: ['正常', '连接异常', '未安装'][index % 3],

  host: `host-${(index % 8) + 1}`,

  source: 'cloud',

  type: ['ecs', 'vm'][index % 2],

  cloudPlatform: `cloud-platform-${(index % 3) + 1}`,

  cloudTag: `tag-${index + 1}`,

  createTime: new Date(Date.now() - index * 86400000).toLocaleString(),

  discoverTime: new Date(Date.now() - index * 3600000).toLocaleString()

}));



const hosts = Array(20).fill(0).map((_, index) => ({

  name: `host-${index + 1}`,

  id: `h-${index + 1}`,

  region: `region-${(index % 5) + 1}`,

  zone: `zone-${(index % 10) + 1}`,

  internalIp: `192.168.${Math.floor(index / 256)}.${index % 256}`,

  externalIp: index % 2 === 0 ? `203.0.${Math.floor(index / 256)}.${index % 256}` : '',

  cpu: 8 + index % 16,

  memory: 32 + index % 64,

  nicCount: 2 + index % 4,

  collectorStatus: ['正常', '连接异常', '未安装'][index % 3],

  createTime: new Date(Date.now() - index * 86400000).toLocaleString(),

  discoverTime: new Date(Date.now() - index * 3600000).toLocaleString()

}));



// 主机

const cloudServerTotal = ref(cloudServers.length);

const hostTotal = ref(hosts.length);



// 新建规则

const createCloudServer = () => {

  };



const exportCloudServerCSV = () => {

  };



const viewAllNics = () => {

  };



const refreshCache = () => {

  };



const handleComputeSearch = () => {

  };



const viewVpc = (row: any) => {

  };



const getStatusType = (status: string): string => {

  const typeMap: Record<string, string> = {

    '正常': 'success',

    '连接异常': 'danger',

    '未安装': 'warning'

  };

  return typeMap[status] || '';

};



const editCloudServer = (row: any) => {

  };



const viewNics = (row: any) => {

  };



const viewCollectorStatus = (row: any) => {

  };



const createHost = () => {

  };



const exportHostCSV = () => {

  };



const viewHost = (row: any) => {

  };



const editHost = (row: any) => {

  };



const handleCurrentChange = (page: number) => {

  currentPage.value = page;

  };

</script>



<style scoped>

.compute-resource-content {

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



.action-buttons {

  display: flex;

  gap: 10px;

  margin-bottom: 20px;

}



.search-filter {

  background-color: white;

  padding: 15px;

  border-radius: 4px;

  margin-bottom: 20px;

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