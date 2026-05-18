<template>

  <div class="network-resource-content">

 <!-- 网络资源 -->

    <div class="card-header">

      <h2>网络资源</h2>

    </div>

    

 <!-- 标签页-->

    <el-tabs v-model="activeNetworkTab">

 <!-- VPC标签页-->

      <el-tab-pane label="VPC" name="vpc">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button type="primary" @click="createVpc">新建/编辑VPC</el-button>

          <el-button @click="exportVpcCSV">导出数据库 CSV</el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="networkSearchForm" class="demo-form-inline">

            <el-form-item label="VPC列表（共16条）">

              <el-select v-model="networkSearchForm.filter" placeholder="全部">

                <el-option label="全部" value="all" />

              </el-select>

            </el-form-item>

            <el-form-item label="搜索">

              <el-input v-model="networkSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleNetworkSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- VPC列表 -->

        <el-table :data="vpcs" style="width: 100%">

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="id" label="ID" width="150" />

          <el-table-column prop="region" label="区域" width="120" />

          <el-table-column prop="subnetCount" label="可用子网数量" width="100" />

          <el-table-column prop="serverCount" label="云服务器列表" width="120" />

          <el-table-column prop="podCount" label="POD数量" width="100" />

          <el-table-column prop="externalIpCount" label="外网IP数量" width="120" />

          <el-table-column prop="cidr" label="CIDR" width="150" />

          <el-table-column prop="cloudPlatform" label="云平台/可用区" width="150" />

          <el-table-column prop="source" label="来源" width="100" />

          <el-table-column prop="discoverTime" label="发现时间" width="150" />

          <el-table-column label="操作" width="100" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="editVpc(scope.row)">

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

            :total="vpcTotal"

            :page-size="pageSize"

            :current-page="currentPage"

            @current-change="handleCurrentChange"

          />

        </div>

      </el-tab-pane>

      

 <!-- 可用子网详情-->

      <el-tab-pane label="子网" name="subnet">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button type="primary" @click="createSubnet">新建子网</el-button>

          <el-button @click="exportSubnetCSV">导出数据库 CSV</el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="networkSearchForm" class="demo-form-inline">

            <el-form-item label="子网列表（共47条）">

              <el-select v-model="networkSearchForm.filter" placeholder="全部">

                <el-option label="全部" value="all" />

              </el-select>

            </el-form-item>

            <el-form-item label="搜索">

              <el-input v-model="networkSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleNetworkSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 子网列表 -->

        <el-table :data="subnets" style="width: 100%">

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="region" label="区域" width="120" />

          <el-table-column prop="vpc" label="VPC" width="150" />

          <el-table-column prop="type" label="类型" width="100" />

          <el-table-column prop="cidr" label="CIDR" width="150" />

          <el-table-column prop="serverCount" label="云服务器列表" width="120" />

          <el-table-column prop="podCount" label="POD数量" width="100" />

          <el-table-column prop="associatedRouterCount" label="关联路由数量" width="150" />

          <el-table-column prop="usedIpCount" label="已使用IP数量" width="120" />

          <el-table-column prop="source" label="来源" width="100" />

          <el-table-column prop="discoverTime" label="发现时间" width="150" />

          <el-table-column label="操作" width="100" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="editSubnet(scope.row)">

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

            :total="subnetTotal"

            :page-size="pageSize"

            :current-page="currentPage"

            @current-change="handleCurrentChange"

          />

        </div>

      </el-tab-pane>

      

 <!-- 路由器标签页 -->

      <el-tab-pane label="路由器" name="router">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button @click="exportRouterCSV">导出数据库 CSV</el-button>

          <el-button @click="viewAllRouteRules">全部路径规则</el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="networkSearchForm" class="demo-form-inline">

            <el-form-item label="路由器列表(可展开)">

              <el-select v-model="networkSearchForm.filter" placeholder="全部">

                <el-option label="全部" value="all" />

              </el-select>

            </el-form-item>

            <el-form-item label="搜索">

              <el-input v-model="networkSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleNetworkSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- 路由器列表-->

        <el-table :data="routers" style="width: 100%">

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="region" label="区域" width="120" />

          <el-table-column prop="vpc" label="VPC" width="150" />

          <el-table-column prop="associatedSubnetCount" label="关联子网数量" width="150" />

          <el-table-column prop="internalIp" label="内网IP" width="150" />

          <el-table-column prop="externalIp" label="外网IP" width="150" />

          <el-table-column prop="routeRuleCount" label="路由规则数量" width="150" />

          <el-table-column prop="cloudPlatform" label="云平台/可用区" width="150" />

          <el-table-column prop="discoverTime" label="发现时间" width="150" />

          <el-table-column label="操作" width="100" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="viewRouteRules(scope.row)">

                <el-icon><View /></el-icon>

              </el-button>

            </template>

          </el-table-column>

        </el-table>

        

 <!-- 数据库表 -->

        <div class="pagination mt-4">

          <el-pagination

            background

            layout="prev, pager, next, jumper"

            :total="routerTotal"

            :page-size="pageSize"

            :current-page="currentPage"

            @current-change="handleCurrentChange"

          />

        </div>

      </el-tab-pane>

      

 <!-- DHCP服务详情-->

      <el-tab-pane label="DHCP网关" name="dhcp">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button type="primary" @click="createDhcpGateway">新建/编辑DHCP网关</el-button>

          <el-button @click="exportDhcpGatewayCSV">导出数据库 CSV</el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="networkSearchForm" class="demo-form-inline">

            <el-form-item label="搜索">

              <el-input v-model="networkSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleNetworkSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- DHCP网关列表 -->

        <el-table :data="dhcpGateways" style="width: 100%">

          <el-table-column prop="name" label="名称" min-width="150" />

          <el-table-column prop="region" label="区域" width="120" />

          <el-table-column prop="vpc" label="VPC" width="150" />

          <el-table-column prop="ip" label="IP" width="150" />

          <el-table-column prop="cloudPlatform" label="云平台/可用区" width="150" />

          <el-table-column prop="deleteTime" label="删除时间" width="150" />

          <el-table-column label="操作" width="100" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="editDhcpGateway(scope.row)">

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

            :total="dhcpGatewayTotal"

            :page-size="pageSize"

            :current-page="currentPage"

            @current-change="handleCurrentChange"

          />

        </div>

      </el-tab-pane>

      

 <!-- IP管理详情-->

      <el-tab-pane label="IP地址管理" name="ip">

 <!-- 操作按钮区域 -->

        <div class="action-buttons mb-4">

          <el-button @click="exportIpCSV">导出数据库 CSV</el-button>

          <el-button @click="refreshIpCache">

            <el-icon><Refresh /></el-icon> 刷新采集器状态

          </el-button>

        </div>

        

 <!-- 最近5分钟 -->

        <div class="search-filter mb-4">

          <el-form :inline="true" :model="networkSearchForm" class="demo-form-inline">

            <el-form-item label="IP地址列表（共2488条）">

              <el-select v-model="networkSearchForm.filter" placeholder="全部">

                <el-option label="全部" value="all" />

              </el-select>

            </el-form-item>

            <el-form-item label="搜索">

              <el-input v-model="networkSearchForm.keyword" placeholder="搜索" />

            </el-form-item>

            <el-form-item>

              <el-button type="primary" @click="handleNetworkSearch">搜索</el-button>

            </el-form-item>

          </el-form>

        </div>

        

 <!-- IP地址列表 -->

        <el-table :data="ipAddresses" style="width: 100%">

          <el-table-column prop="ip" label="IP" width="150" />

          <el-table-column prop="type" label="类型" width="100" />

          <el-table-column prop="region" label="区域" width="120" />

          <el-table-column prop="vpc" label="VPC" width="150" />

          <el-table-column prop="mac" label="MAC" width="150" />

          <el-table-column prop="associatedResourceType" label="关联资源类型" width="150" />

          <el-table-column prop="associatedResource" label="关联资源" width="150" />

          <el-table-column prop="gatewayIp" label="网关IP" width="150" />

          <el-table-column prop="cloudPlatform" label="云平台/可用区" width="150" />

          <el-table-column label="操作" width="100" fixed="right">

            <template #default="scope">

              <el-button size="small" @click="viewIpDetail(scope.row)">

                <el-icon><View /></el-icon>

              </el-button>

            </template>

          </el-table-column>

        </el-table>

        

 <!-- 数据库表 -->

        <div class="pagination mt-4">

          <el-pagination

            background

            layout="prev, pager, next, jumper"

            :total="ipAddressTotal"

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

import { Edit, View, Refresh } from '@element-plus/icons-vue';



// 标签页

const activeNetworkTab = ref('vpc');



// 表单流量详情

const networkSearchForm = reactive({

  filter: 'all',

  keyword: ''

});



// 数据库表详情

const currentPage = ref(1);

const pageSize = ref(10);



// 模拟流量详情

const vpcs = Array(16).fill(0).map((_, index) => ({

  name: `vpc-${index + 1}`,

  id: `vpc-${index + 1}`,

  region: `region-${(index % 5) + 1}`,

  subnetCount: 5,

  serverCount: 25,

  podCount: 250,

  externalIpCount: 10,

  cidr: `10.${index}.0.0/16`,

  cloudPlatform: `cloud-platform-${(index % 3) + 1}`,

  source: 'cloud',

  discoverTime: new Date(Date.now() - index * 3600000).toLocaleString()

}));



const subnets = Array(47).fill(0).map((_, index) => ({

  name: `subnet-${index + 1}`,

  region: `region-${(index % 5) + 1}`,

  vpc: `vpc-${(index % 16) + 1}`,

  type: ['public', 'private'][index % 2],

  cidr: `10.${Math.floor(index / 256)}.${index % 256}.0/24`,

  serverCount: 10,

  podCount: 100,

  associatedRouterCount: 3,

  usedIpCount: 127,

  source: 'cloud',

  discoverTime: new Date(Date.now() - index * 3600000).toLocaleString()

}));



const routers = Array(3).fill(0).map((_, index) => ({

  name: `router-${index + 1}`,

  region: `region-${(index % 5) + 1}`,

  vpc: `vpc-${(index % 16) + 1}`,

  associatedSubnetCount: 5,

  internalIp: `10.0.0.${index + 1}`,

  externalIp: `203.0.113.${index + 1}`,

  routeRuleCount: 10,

  cloudPlatform: `cloud-platform-${(index % 3) + 1}`,

  discoverTime: new Date(Date.now() - index * 3600000).toLocaleString()

}));



const dhcpGateways = Array(10).fill(0).map((_, index) => ({

  name: `dhcp-${index + 1}`,

  region: `region-${(index % 5) + 1}`,

  vpc: `vpc-${(index % 16) + 1}`,

  ip: `10.${Math.floor(index / 256)}.${index % 256}.1`,

  cloudPlatform: `cloud-platform-${(index % 3) + 1}`,

  deleteTime: index % 3 === 0 ? new Date(Date.now() - index * 86400000).toLocaleString() : ''

}));



const ipAddresses = Array(20).fill(0).map((_, index) => ({

  ip: `10.0.${Math.floor(index / 256)}.${index % 256}`,

  type: ['private', 'public'][index % 2],

  region: `region-${(index % 5) + 1}`,

  vpc: `vpc-${(index % 16) + 1}`,

  mac: `${aa}:${aa}:${aa}:${aa}:${aa}:${aa}`,

  associatedResourceType: ['server', 'pod', 'router'][index % 3],

  associatedResource: `resource-${index + 1}`,

  gatewayIp: `10.0.${Math.floor(index / 256)}.1`,

  cloudPlatform: `cloud-platform-${(index % 3) + 1}`

}));



// 主机

const vpcTotal = ref(vpcs.length);

const subnetTotal = ref(subnets.length);

const routerTotal = ref(routers.length);

const dhcpGatewayTotal = ref(dhcpGateways.length);

const ipAddressTotal = ref(2488);



// 新建规则

const createVpc = () => {

  };



const exportVpcCSV = () => {

  };



const handleNetworkSearch = () => {

  };



const editVpc = (row: any) => {

  };



const createSubnet = () => {

  };



const exportSubnetCSV = () => {

  };



const editSubnet = (row: any) => {

  };



const exportRouterCSV = () => {

  };



const viewAllRouteRules = () => {

  };



const viewRouteRules = (row: any) => {

  };



const createDhcpGateway = () => {

  };



const exportDhcpGatewayCSV = () => {

  };



const editDhcpGateway = (row: any) => {

  };



const exportIpCSV = () => {

  };



const refreshIpCache = () => {

  };



const viewIpDetail = (row: any) => {

  };



const handleCurrentChange = (page: number) => {

  currentPage.value = page;

  };

</script>



<style scoped>

.network-resource-content {

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