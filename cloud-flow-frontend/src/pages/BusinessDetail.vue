<template>

  <div class="business-detail">

    <div class="detail-header">

      <h3>{{ businessInfo.name }} - 业务详情</h3>

      <div class="header-actions">

        <el-button @click="viewTopology">

          服务拓扑

        </el-button>

        <el-button @click="viewServiceList">

          服务列表

        </el-button>

      </div>

    </div>

    <div class="detail-stats">

      <el-card class="stat-card">

        <div class="stat-content">

          <div class="stat-number" @click="activeTab = 'services'">{{ businessInfo.services }}</div>

          <div class="stat-label">服务</div>

        </div>

      </el-card>

      <el-card class="stat-card">

        <div class="stat-content">

          <div class="stat-number" @click="activeTab = 'serviceGroups'">{{ businessInfo.serviceGroups }}</div>

          <div class="stat-label">服务组</div>

        </div>

      </el-card>

      <el-card class="stat-card">

        <div class="stat-content">

          <div class="stat-number" @click="activeTab = 'paths'">{{ businessInfo.paths }}</div>

          <div class="stat-label">路径</div>

        </div>

      </el-card>

    </div>

    <div class="detail-tabs">

      <el-tabs v-model="activeTab">

        <el-tab-pane label="服务" name="services">

          <div class="tab-header">

            <h4>服务列表</h4>

            <el-button type="primary" @click="createService">

              新建服务

            </el-button>

          </div>

          <el-table :data="servicesData" style="width: 100%">

            <el-table-column prop="name" label="名称" width="180" />

            <el-table-column prop="legend" label="图例" width="120" />

            <el-table-column prop="filterType" label="过滤类型" width="120" />

            <el-table-column prop="serviceGroup" label="服务端" width="150" />

            <el-table-column prop="metrics" label="指标阈值" />

            <el-table-column label="操作" width="150" fixed="right">

              <template #default="scope">

                <el-button size="small" @click="editService(scope.row)">

                  编辑

                </el-button>

                <el-button size="small" type="danger" @click="confirmDeleteService(scope.row.id)">

                  删除

                </el-button>

              </template>

            </el-table-column>

          </el-table>

        </el-tab-pane>

        <el-tab-pane label="服务组" name="serviceGroups">

          <div class="tab-header">

            <h4>服务组列表</h4>

            <el-button type="primary" @click="createServiceGroup">

              新建服务?

            </el-button>

          </div>

          <el-table :data="serviceGroupsData" style="width: 100%">

            <el-table-column prop="name" label="名称" width="180" />

            <el-table-column prop="type" label="类型" width="120" />

            <el-table-column prop="filter" label="过滤条件" />

            <el-table-column label="操作" width="150" fixed="right">

              <template #default="scope">

                <el-button size="small" @click="editServiceGroup(scope.row)">

                  编辑

                </el-button>

                <el-button size="small" type="danger" @click="confirmDeleteServiceGroup(scope.row.id)">

                  删除

                </el-button>

              </template>

            </el-table-column>

          </el-table>

        </el-tab-pane>

        <el-tab-pane label="路径" name="paths">

          <div class="tab-header">

            <h4>路径列表</h4>

            <el-button type="primary" @click="createPath">

              新建路径

            </el-button>

            <el-button type="danger" @click="batchDeletePaths" :disabled="selectedPaths.length === 0">

              批量删除

            </el-button>

          </div>

          <el-table 

            :data="pathsData" 

            style="width: 100%"

            @selection-change="handlePathSelectionChange"

          >

            <el-table-column type="selection" width="50" />

            <el-table-column prop="name" label="名称" width="180" />

            <el-table-column prop="client" label="客户端" width="150" />

            <el-table-column prop="server" label="服务端" width="150" />

            <el-table-column label="操作" width="150" fixed="right">

              <template #default="scope">

                <el-button size="small" @click="editPath(scope.row)">

                  编辑

                </el-button>

                <el-button size="small" type="danger" @click="confirmDeletePath(scope.row.id)">

                  删除

                </el-button>

              </template>

            </el-table-column>

          </el-table>

        </el-tab-pane>

      </el-tabs>

    </div>

    

 <!-- 新建/编辑服务对话框 -->

    <el-dialog

      v-model="serviceDialogVisible"

      :title="serviceDialogTitle"

      width="600px"

    >

      <el-form :model="serviceForm" label-width="100px" :rules="serviceRules" ref="serviceFormRef">

        <el-form-item label="服务名称" prop="name" required>

          <el-input v-model="serviceForm.name" placeholder="请输入服务名" />

        </el-form-item>

        <el-form-item label="图例" prop="legend" required>

          <el-input v-model="serviceForm.legend" placeholder="请输入图例" />

        </el-form-item>

        <el-form-item label="过滤类型">

          <el-radio-group v-model="serviceForm.filterType">

            <el-radio label="双向">双向</el-radio>

            <el-radio label="单向">单向</el-radio>

          </el-radio-group>

        </el-form-item>

        <el-form-item label="服务组">

          <el-select v-model="serviceForm.serviceGroup" placeholder="请选择服务组" style="width: 100%">

            <el-option label="服务端" value="group1" />

            <el-option label="服务端" value="group2" />

            <el-option label="自定义服务组" value="custom" />

          </el-select>

        </el-form-item>

        <el-form-item label="指标阈值">

          <el-input v-model="serviceForm.metrics" placeholder="请设置指标阈值" />

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="serviceDialogVisible = false">取消</el-button>

          <el-button type="primary" @click="saveService" :disabled="!isServiceFormValid">保存</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 新建/编辑服务组对话框 -->

    <el-dialog

      v-model="serviceGroupDialogVisible"

      :title="serviceGroupDialogTitle"

      width="600px"

    >

      <el-form :model="serviceGroupForm" label-width="100px" :rules="serviceGroupRules" ref="serviceGroupFormRef">

        <el-form-item label="服务组名称" prop="name" required>

          <el-input v-model="serviceGroupForm.name" placeholder="请输入服务组名称" />

        </el-form-item>

        <el-form-item label="类型">

          <el-radio-group v-model="serviceGroupForm.type">

            <el-radio label="自动分组">自动分组</el-radio>

            <el-radio label="自定义">自定义</el-radio>

          </el-radio-group>

        </el-form-item>

        <el-form-item label="过滤条件" v-if="serviceGroupForm.type === '自动分组'">

          <el-input v-model="serviceGroupForm.filter" placeholder="请输入过滤条件" />

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="serviceGroupDialogVisible = false">取消</el-button>

          <el-button type="primary" @click="saveServiceGroup" :disabled="!isServiceGroupFormValid">保存</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 新建/编辑路径对话框 -->

    <el-dialog

      v-model="pathDialogVisible"

      :title="pathDialogTitle"

      width="600px"

    >

      <el-form :model="pathForm" label-width="100px" :rules="pathRules" ref="pathFormRef">

        <el-form-item label="路径名称" prop="name" required>

          <el-input v-model="pathForm.name" placeholder="请输入路径名称" />

        </el-form-item>

        <el-form-item label="客户端">

          <el-select

            v-model="pathForm.client"

            multiple

            placeholder="请选择客户端"

            style="width: 100%"

          >

            <el-option label="全部" value="all" />

            <el-option label="服务1" value="service1" />

            <el-option label="服务2" value="service2" />

            <el-option label="服务端" value="group1" />

          </el-select>

        </el-form-item>

        <el-form-item label="服务组">

          <el-select

            v-model="pathForm.server"

            multiple

            placeholder="请选择服务端"

            style="width: 100%"

          >

            <el-option label="全部" value="all" />

            <el-option label="服务1" value="service1" />

            <el-option label="服务2" value="service2" />

            <el-option label="服务端" value="group1" />

          </el-select>

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="pathDialogVisible = false">取消</el-button>

          <el-button type="primary" @click="savePath" :disabled="!isPathFormValid">保存</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 删除确认对话框 -->

    <el-dialog

      v-model="deleteDialogVisible"

      title="确认删除"

      width="400px"

    >

      <p>{{ deleteMessage }}</p>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="deleteDialogVisible = false">取消</el-button>

          <el-button type="danger" @click="deleteItem">删除</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

// TODO: 当前页面数据库均为 mock 数据库，待后端 API 就绪后替换为真实接口调用
import { ref, computed } from 'vue'

import { useRouter, useRoute } from 'vue-router'



const router = useRouter()

const route = useRoute()



// 业务信息

const businessInfo = ref({

  id: 1,

  name: '电商业务',

  team: '团队A',

  table: 'table1',

  services: 5,

  serviceGroups: 2,

  paths: 3,

  creator: 'admin',

  updateTime: '2023-09-01 10:00:00'

})



// 活跃标签

const activeTab = ref('services')



// 服务数据库

const servicesData = ref([

  {

    id: 1,

    name: 'web-shop',

    legend: 'web-shop',

    filterType: '双向',

    serviceGroup: '服务',

    metrics: '响应时间>100ms'

  },

  {

    id: 2,

    name: 'svc-user',

    legend: 'svc-user',

    filterType: '双向',

    serviceGroup: '服务',

    metrics: '错误率1%'

  },

  {

    id: 3,

    name: 'svc-order',

    legend: 'svc-order',

    filterType: '单向',

    serviceGroup: '服务',

    metrics: '响应时间>50ms'

  },

  {

    id: 4,

    name: 'svc-payment',

    legend: 'svc-payment',

    filterType: '双向',

    serviceGroup: '服务',

    metrics: '响应时间>200ms'

  },

  {

    id: 5,

    name: 'svc-shipping',

    legend: 'svc-shipping',

    filterType: '单向',

    serviceGroup: '自定义服务组',

    metrics: '响应时间>150ms'

  }

])



// 服务组数据库

const serviceGroupsData = ref([

  {

    id: 1,

    name: '服务',

    type: '自动分组',

    filter: 'service=web-shop,svc-user'

  },

  {

    id: 2,

    name: '服务',

    type: '自定义',

    filter: ''

  }

])



// 路径数据库

const pathsData = ref([

  {

    id: 1,

    name: '路径1',

    client: 'web-shop',

    server: 'svc-user'

  },

  {

    id: 2,

    name: '路径2',

    client: 'web-shop',

    server: 'svc-order'

  },

  {

    id: 3,

    name: '路径3',

    client: 'svc-order',

    server: 'svc-payment'

  }

])



// 选中的路径

const selectedPaths = ref([])



// 服务对话框数据库

const serviceDialogVisible = ref(false)

const serviceDialogTitle = ref('新建服务')

const serviceFormRef = ref()

const serviceForm = ref({

  id: '',

  name: '',

  legend: '',

  filterType: '双向',

  serviceGroup: '',

  metrics: ''

})



const serviceRules = ref({

  name: [{ required: true, message: '请输入服务名', trigger: 'blur' }],

  legend: [{ required: true, message: '请输入图例', trigger: 'blur' }]

})



// 服务组对话框数据库

const serviceGroupDialogVisible = ref(false)

const serviceGroupDialogTitle = ref('新建服务组')

const serviceGroupFormRef = ref()

const serviceGroupForm = ref({

  id: '',

  name: '',

  type: '自动分组',

  filter: ''

})



const serviceGroupRules = ref({

  name: [{ required: true, message: '请输入服务组名称', trigger: 'blur' }]

})



// 路径对话框数据库

const pathDialogVisible = ref(false)

const pathDialogTitle = ref('新建路径')

const pathFormRef = ref()

const pathForm = ref({

  id: '',

  name: '',

  client: [],

  server: []

})



const pathRules = ref({

  name: [{ required: true, message: '请输入路径名称', trigger: 'blur' }]

})



// 删除确认对话框

const deleteDialogVisible = ref(false)

const deleteMessage = ref('确定要删除吗？')

const deleteType = ref('')

const selectedItemId = ref(0)



// 表单验证

const isServiceFormValid = computed(() => {

  return serviceForm.value.name && serviceForm.value.legend

})



const isServiceGroupFormValid = computed(() => {

  return serviceGroupForm.value.name

})



const isPathFormValid = computed(() => {

  return pathForm.value.name

})



// 查看服务拓扑

const viewTopology = () => {

  router.push(`/business/topology/${businessInfo.value.id}`)

  }



// 查看服务列表

const viewServiceList = () => {

  router.push(`/business/services/${businessInfo.value.id}`)

  }



// 创建服务

const createService = () => {

  serviceDialogTitle.value = '新建服务'

  serviceForm.value = {

    id: '',

    name: '',

    legend: '',

    filterType: '双向',

    serviceGroup: '',

    metrics: ''

  }

  serviceDialogVisible.value = true

}



// 编辑服务

const editService = (row: any) => {

  serviceDialogTitle.value = '编辑服务'

  serviceForm.value = {

    id: row.id,

    name: row.name,

    legend: row.legend,

    filterType: row.filterType,

    serviceGroup: row.serviceGroup,

    metrics: row.metrics

  }

  serviceDialogVisible.value = true

}



// 确认删除服务

const confirmDeleteService = (id: number) => {

  deleteMessage.value = '确定要删除该服务吗？'

  deleteType.value = 'service'

  selectedItemId.value = id

  deleteDialogVisible.value = true

}



// 创建服务组

const createServiceGroup = () => {

  serviceGroupDialogTitle.value = '新建服务组'

  serviceGroupForm.value = {

    id: '',

    name: '',

    type: '自动分组',

    filter: ''

  }

  serviceGroupDialogVisible.value = true

}



// 编辑服务组

const editServiceGroup = (row: any) => {

  serviceGroupDialogTitle.value = '编辑服务组'

  serviceGroupForm.value = {

    id: row.id,

    name: row.name,

    type: row.type,

    filter: row.filter

  }

  serviceGroupDialogVisible.value = true

}



// 确认删除服务组

const confirmDeleteServiceGroup = (id: number) => {

  deleteMessage.value = '确定要删除该服务组吗？'

  deleteType.value = 'serviceGroup'

  selectedItemId.value = id

  deleteDialogVisible.value = true

}



// 创建路径

const createPath = () => {

  pathDialogTitle.value = '新建路径'

  pathForm.value = {

    id: '',

    name: '',

    client: [],

    server: []

  }

  pathDialogVisible.value = true

}



// 编辑路径

const editPath = (row: any) => {

  pathDialogTitle.value = '编辑路径'

  pathForm.value = {

    id: row.id,

    name: row.name,

    client: [row.client],

    server: [row.server]

  }

  pathDialogVisible.value = true

}



// 确认删除路径

const confirmDeletePath = (id: number) => {

  deleteMessage.value = '确定要删除该路径吗？'

  deleteType.value = 'path'

  selectedItemId.value = id

  deleteDialogVisible.value = true

}



// 批量删除路径

const batchDeletePaths = () => {

  if (selectedPaths.value.length > 0) {

    deleteMessage.value = `确定要删除选中的${selectedPaths.value.length}个路径吗？`

    deleteType.value = 'batchPath'

    deleteDialogVisible.value = true

  }

}



// 处理路径选择变化

const handlePathSelectionChange = (val: any[]) => {

  selectedPaths.value = val

}



// 删除项目

const deleteItem = () => {

  switch (deleteType.value) {

    case 'service':

      servicesData.value = servicesData.value.filter(item => item.id !== selectedItemId.value)

      businessInfo.value.services = servicesData.value.length

      break

    case 'serviceGroup':

      serviceGroupsData.value = serviceGroupsData.value.filter(item => item.id !== selectedItemId.value)

      businessInfo.value.serviceGroups = serviceGroupsData.value.length

      break

    case 'path':

      pathsData.value = pathsData.value.filter(item => item.id !== selectedItemId.value)

      businessInfo.value.paths = pathsData.value.length

      break

    case 'batchPath':

      const selectedIds = selectedPaths.value.map(item => item.id)

      pathsData.value = pathsData.value.filter(item => !selectedIds.includes(item.id))

      businessInfo.value.paths = pathsData.value.length

      selectedPaths.value = []

      break

  }

  deleteDialogVisible.value = false

  }



// 保存服务

const saveService = () => {

  if (serviceForm.value.id) {

 // 编辑

    const index = servicesData.value.findIndex(item => item.id === serviceForm.value.id)

    if (index !== -1) {

      servicesData.value[index] = {

        ...servicesData.value[index],

        name: serviceForm.value.name,

        legend: serviceForm.value.legend,

        filterType: serviceForm.value.filterType,

        serviceGroup: serviceForm.value.serviceGroup,

        metrics: serviceForm.value.metrics

      }

    }

  } else {

 // 新建

    const newService = {

      id: servicesData.value.length + 1,

      name: serviceForm.value.name,

      legend: serviceForm.value.legend,

      filterType: serviceForm.value.filterType,

      serviceGroup: serviceForm.value.serviceGroup,

      metrics: serviceForm.value.metrics

    }

    servicesData.value.push(newService)

    businessInfo.value.services = servicesData.value.length

  }

  serviceDialogVisible.value = false

  }



// 保存服务组

const saveServiceGroup = () => {

  if (serviceGroupForm.value.id) {

 // 编辑

    const index = serviceGroupsData.value.findIndex(item => item.id === serviceGroupForm.value.id)

    if (index !== -1) {

      serviceGroupsData.value[index] = {

        ...serviceGroupsData.value[index],

        name: serviceGroupForm.value.name,

        type: serviceGroupForm.value.type,

        filter: serviceGroupForm.value.filter

      }

    }

  } else {

 // 新建

    const newServiceGroup = {

      id: serviceGroupsData.value.length + 1,

      name: serviceGroupForm.value.name,

      type: serviceGroupForm.value.type,

      filter: serviceGroupForm.value.filter

    }

    serviceGroupsData.value.push(newServiceGroup)

    businessInfo.value.serviceGroups = serviceGroupsData.value.length

  }

  serviceGroupDialogVisible.value = false

  }



// 保存路径

const savePath = () => {

  if (pathForm.value.id) {

 // 编辑

    const index = pathsData.value.findIndex(item => item.id === pathForm.value.id)

    if (index !== -1) {

      pathsData.value[index] = {

        ...pathsData.value[index],

        name: pathForm.value.name,

        client: pathForm.value.client.join(','),

        server: pathForm.value.server.join(',')

      }

    }

  } else {

 // 新建

    const newPath = {

      id: pathsData.value.length + 1,

      name: pathForm.value.name,

      client: pathForm.value.client.join(','),

      server: pathForm.value.server.join(',')

    }

    pathsData.value.push(newPath)

    businessInfo.value.paths = pathsData.value.length

  }

  pathDialogVisible.value = false

  }

</script>



<style scoped>

.business-detail {

  padding: 24px;

}



.detail-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 24px;

}



.detail-header h3 {

  margin: 0;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.header-actions {

  display: flex;

  gap: 10px;

}



.detail-stats {

  display: flex;

  gap: 24px;

  margin-bottom: 24px;

}



.stat-card {

  flex: 1;

  cursor: pointer;

  transition: all 0.3s ease;

}



.stat-card:hover {

  transform: translateY(-2px);

  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);

}



.stat-content {

  display: flex;

  flex-direction: column;

  align-items: center;

  padding: 20px;

}



.stat-number {

  font-size: 24px;

  font-weight: bold;

  color: #1677FF;

  margin-bottom: 8px;

}



.stat-label {

  font-size: 14px;

  color: #606266;

}



.detail-tabs {

  background-color: white;

  border-radius: 4px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  padding: 24px;

}



.tab-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 24px;

}



.tab-header h4 {

  margin: 0;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.dialog-footer {

  display: flex;

  justify-content: flex-end;

  gap: 10px;

}



@media (max-width: 768px) {

  .detail-header {

    flex-direction: column;

    align-items: flex-start;

    gap: 10px;

  }

  

  .detail-stats {

    flex-direction: column;

  }

  

  .tab-header {

    flex-direction: column;

    align-items: flex-start;

    gap: 10px;

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

</style>