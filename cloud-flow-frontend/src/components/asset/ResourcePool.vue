<template>

  <div class="resource-pool-content">

 <!-- 资源池-->

    <div class="card-header">

      <h2>资源池</h2>

    </div>

    

 <!-- 标签页-->

    <el-tabs v-model="activeResourcePoolTab">

 <!-- 云平台管理页面 -->

      <el-tab-pane label="云平台/可用区" name="cloud-platform">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button type="primary" @click="createCloudPlatform">新建云平台</el-button>

          <el-button @click="exportCloudPlatformCSV">导出数据库 CSV</el-button>

          <el-button @click="configureSyncInterval">配置同步间隔</el-button>

          <el-button @click="showSyncTimeStats">同步时间范围</el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="resourcePoolSearchForm" class="demo-form-inline">

            <el-form-item label="搜索">

              <el-input v-model="resourcePoolSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleResourcePoolSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 云平台列表-->

        <el-table :data="cloudPlatforms" style="width: 100%">

          <el-table-column type="expand">

            <template #default="scope">

              <div class="expand-content">

                <p><strong>所属国家/城市:</strong> {{ scope.row.location }}</p>

                <p><strong>资源同步控制器</strong> {{ scope.row.syncController }}</p>

                <p><strong>资源同步错误信息:</strong> {{ scope.row.syncError }}</p>

                <p><strong>POD子网IPv4地址最大值:</strong> {{ scope.row.podSubnetIpv4Max }}</p>

                <p><strong>POD子网IPv6地址最大值:</strong> {{ scope.row.podSubnetIpv6Max }}</p>

              </div>

            </template>

          </el-table-column>

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="id" label="ID" width="150" />

          <el-table-column prop="type" label="类型" width="120" />

          <el-table-column prop="regionCount" label="区域数量" width="100">

            <template #default="scope">

              <el-button  @click="viewRegions(scope.row)">{{ scope.row.regionCount }}</el-button>

            </template>

          </el-table-column>

          <el-table-column prop="zoneCount" label="可用区数量" width="120">

            <template #default="scope">

              <el-button  @click="viewZones(scope.row)">{{ scope.row.zoneCount }}</el-button>

            </template>

          </el-table-column>

          <el-table-column prop="containerClusterCount" label="所属容器集群" width="120" />

          <el-table-column prop="syncController" label="资源同步控制器" width="150" />

          <el-table-column prop="syncTime" label="最近同步时间" width="150" />

          <el-table-column prop="syncInterval" label="同步间隔" width="100" />

          <el-table-column prop="application" label="应用名称" width="100" />

          <el-table-column prop="status" label="状态" width="100">

            <template #default="scope">

              <el-switch v-model="scope.row.status" @change="toggleCloudPlatformStatus(scope.row)" />

            </template>

          </el-table-column>

          <el-table-column label="操作" width="150" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="editCloudPlatform(scope.row)">

                <el-icon><Edit /></el-icon>

              </el-button>

              <el-button size="small" type="danger" @click="deleteCloudPlatform(scope.row)">

                <el-icon><Delete /></el-icon>

              </el-button>

            </template>

          </el-table-column>

        </el-table>

        

 <!-- 数据库表 -->

        <div class="pagination mt-4">

          <el-pagination

            background

            layout="prev, pager, next, jumper"

            :total="cloudPlatformTotal"

            :page-size="pageSize"

            :current-page="currentPage"

            @current-change="handleCurrentChange"

          />

        </div>

      </el-tab-pane>

      

 <!-- 区域管理详情-->

      <el-tab-pane label="区域" name="region">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button type="primary" @click="createRegion">创建区域管理</el-button>

          <el-button @click="exportRegionCSV">导出数据库 CSV</el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="resourcePoolSearchForm" class="demo-form-inline">

            <el-form-item label="搜索">

              <el-input v-model="resourcePoolSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleResourcePoolSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 区域列表 -->

        <el-table :data="regions" style="width: 100%">

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="zoneCount" label="可用区数量" width="120">

            <template #default="scope">

              <el-button  @click="viewZonesByRegion(scope.row)">{{ scope.row.zoneCount }}</el-button>

            </template>

          </el-table-column>

          <el-table-column prop="vpcCount" label="VPC数量" width="100">

            <template #default="scope">

              <el-button  @click="viewVpcs(scope.row)">{{ scope.row.vpcCount }}</el-button>

            </template>

          </el-table-column>

          <el-table-column prop="subnetCount" label="可用子网数量" width="100">

            <template #default="scope">

              <el-button  @click="viewSubnets(scope.row)">{{ scope.row.subnetCount }}</el-button>

            </template>

          </el-table-column>

          <el-table-column prop="serverCount" label="云服务器列表" width="120">

            <template #default="scope">

              <el-button  @click="viewServers(scope.row)">{{ scope.row.serverCount }}</el-button>

            </template>

          </el-table-column>

          <el-table-column prop="podCount" label="POD数量" width="100">

            <template #default="scope">

              <el-button  @click="viewPods(scope.row)">{{ scope.row.podCount }}</el-button>

            </template>

          </el-table-column>

          <el-table-column prop="latitude" label="经度(经纬度)" width="120" />

          <el-table-column prop="longitude" label="纬度(经纬度)" width="120" />

          <el-table-column prop="source" label="来源" width="100" />

          <el-table-column prop="syncTime" label="搜索更新时间" width="150" />

          <el-table-column label="操作" width="100" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="editRegion(scope.row)">

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

            :total="regionTotal"

            :page-size="pageSize"

            :current-page="currentPage"

            @current-change="handleCurrentChange"

          />

        </div>

      </el-tab-pane>

      

 <!-- 可用区标签页〉 -->

      <el-tab-pane label="可用区" name="zone">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button type="primary" @click="createZone">创建可用区</el-button>

          <el-button @click="exportZoneCSV">导出数据库 CSV</el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="resourcePoolSearchForm" class="demo-form-inline">

            <el-form-item label="搜索">

              <el-input v-model="resourcePoolSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleResourcePoolSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 可用区列表-->

        <el-table :data="zones" style="width: 100%">

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="region" label="区域" width="120" />

          <el-table-column prop="serverCount" label="云服务器列表" width="120" />

          <el-table-column prop="podCount" label="POD数量" width="100" />

          <el-table-column prop="cloudPlatform" label="云平台/可用区" width="150" />

          <el-table-column prop="syncTime" label="搜索更新时间" width="150" />

          <el-table-column prop="source" label="来源" width="100" />

          <el-table-column label="操作" width="150" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="editZone(scope.row)">

                <el-icon><Edit /></el-icon>

              </el-button>

              <el-button size="small" type="danger" @click="deleteZone(scope.row)">

                <el-icon><Delete /></el-icon>

              </el-button>

            </template>

          </el-table-column>

        </el-table>

        

 <!-- 数据库表 -->

        <div class="pagination mt-4">

          <el-pagination

            background

            layout="prev, pager, next, jumper"

            :total="zoneTotal"

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

import { Edit, Delete } from '@element-plus/icons-vue';



// 标签页

const activeResourcePoolTab = ref('cloud-platform');



// 表单流量详情

const resourcePoolSearchForm = reactive({

  keyword: ''

});



// 数据库表详情

const currentPage = ref(1);

const pageSize = ref(10);



// 模拟流量详情

const cloudPlatforms = Array(20).fill(0).map((_, index) => ({

  name: `cloud-platform-${index + 1}`,

  id: `cp-${index + 1}`,

  type: ['public', 'private'][index % 2],

  regionCount: 5,

  zoneCount: 3,

  containerClusterCount: 10,

  syncController: `controller-${index + 1}`,

  syncTime: new Date().toLocaleString(),

  syncInterval: '300s',

  application: 'cloud-flow',

  status: index % 2 === 0,

  location: `Location ${index + 1}`,

  syncError: index % 3 === 0 ? 'Error' : 'None',

  podSubnetIpv4Max: '10.0.0.255',

  podSubnetIpv6Max: 'fd00::ffff'

}));



const regions = Array(30).fill(0).map((_, index) => ({

  name: `region-${index + 1}`,

  zoneCount: 3,

  vpcCount: 5,

  subnetCount: 10,

  serverCount: 50,

  podCount: 500,

  latitude: 39.9 + index * 0.1,

  longitude: 116.4 + index * 0.1,

  source: 'cloud',

  syncTime: new Date().toLocaleString()

}));



const zones = Array(40).fill(0).map((_, index) => ({

  name: `zone-${index + 1}`,

  region: `region-${(index % 10) + 1}`,

  serverCount: 25,

  podCount: 250,

  cloudPlatform: `cloud-platform-${(index % 5) + 1}`,

  syncTime: new Date().toLocaleString(),

  source: 'cloud'

}));



// 主机

const cloudPlatformTotal = ref(cloudPlatforms.length);

const regionTotal = ref(regions.length);

const zoneTotal = ref(zones.length);



// 新建规则

const createCloudPlatform = () => {

  };



const exportCloudPlatformCSV = () => {

  };



const configureSyncInterval = () => {

  };



const showSyncTimeStats = () => {

  };



const handleResourcePoolSearch = () => {

  };



const viewRegions = (row: any) => {

  };



const viewZones = (row: any) => {

  };



const toggleCloudPlatformStatus = (row: any) => {

  };



const editCloudPlatform = (row: any) => {

  };



const deleteCloudPlatform = (row: any) => {

  };



const createRegion = () => {

  };



const exportRegionCSV = () => {

  };



const viewZonesByRegion = (row: any) => {

  };



const viewVpcs = (row: any) => {

  };



const viewSubnets = (row: any) => {

  };



const viewServers = (row: any) => {

  };



const viewPods = (row: any) => {

  };



const editRegion = (row: any) => {

  };



const createZone = () => {

  };



const exportZoneCSV = () => {

  };



const editZone = (row: any) => {

  };



const deleteZone = (row: any) => {

  };



const handleCurrentChange = (page: number) => {

  currentPage.value = page;

  };

</script>



<style scoped>

.resource-pool-content {

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



.expand-content {

  padding: 10px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.expand-content p {

  margin: 5px 0;

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