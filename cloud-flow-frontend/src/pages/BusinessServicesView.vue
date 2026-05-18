<template>

  <div class="business-services-view">

    <div class="services-header">

      <h3>{{ businessName }} - 服务列表</h3>

      <div class="header-actions">

        <el-button @click="goBack">

          返回列表

        </el-button>

      </div>

    </div>

    <div class="services-content">

      <el-table :data="servicesData" style="width: 100%">

        <el-table-column prop="name" label="名称" width="180" />

        <el-table-column prop="legend" label="图例" width="120" />

        <el-table-column prop="filterType" label="过滤类型" width="120" />

        <el-table-column prop="serviceGroup" label="服务分组" width="150" />

        <el-table-column prop="metrics" label="指标阈值" />

        <el-table-column prop="status" label="状态" width="100">

          <template #default="scope">

            <el-tag :type="scope.row.status === '正常' ? 'success' : 'danger'">

              {{ scope.row.status }}

            </el-tag>

          </template>

        </el-table-column>

        <el-table-column label="操作" width="150" fixed="right">

          <template #default="scope">

            <el-button size="small" @click="editService(scope.row)">

              编辑

            </el-button>

            <el-button size="small" type="danger" @click="confirmDeleteService(scope.row.id)">

              分组聚合网络

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

    

 <!-- 新建/编辑服务对话框-->

    <el-dialog

      v-model="serviceDialogVisible"

      :title="serviceDialogTitle"

      width="600px"

    >

      <el-form :model="serviceForm" label-width="100px" :rules="serviceRules" ref="serviceFormRef">

        <el-form-item label="服务名称" prop="name" required>

          <el-input v-model="serviceForm.name" placeholder="请输入服务名称" />

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

        <el-form-item label="服务分组">

          <el-select v-model="serviceForm.serviceGroup" placeholder="请选择服务分组" style="width: 100%">

            <el-option label="服务分组" value="group1" />

            <el-option label="服务分组" value="group2" />

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

          <el-button type="primary" @click="saveService" :disabled="!isServiceFormValid">保存搜索条件</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 删除确认对话框 -->

    <el-dialog

      v-model="deleteDialogVisible"

      title="确认删除操作"

      width="400px"

    >

      <p>确定要删除该服务详情吗？</p>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="deleteDialogVisible = false">取消</el-button>

          <el-button type="danger" @click="deleteService">删除</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

import { ref, computed } from 'vue'

import { useRouter, useRoute } from 'vue-router'



const router = useRouter()

const route = useRoute()



// 业务策略

const businessName = ref('电商业务')



// 服务数据库流

const servicesData = ref([

  {

    id: 1,

    name: 'web-shop',

    legend: 'web-shop',

    filterType: '双向',

    serviceGroup: '服务分组',

    metrics: '分组聚合详情>100ms',

    status: '正常'

  },

  {

    id: 2,

    name: 'svc-user',

    legend: 'svc-user',

    filterType: '双向',

    serviceGroup: '服务分组',

    metrics: '错误率1%',

    status: '正常'

  },

  {

    id: 3,

    name: 'svc-order',

    legend: 'svc-order',

    filterType: '单向',

    serviceGroup: '服务分组',

    metrics: '分组聚合详情>50ms',

    status: '正常'

  },

  {

    id: 4,

    name: 'svc-payment',

    legend: 'svc-payment',

    filterType: '双向',

    serviceGroup: '服务分组',

    metrics: '分组聚合详情>200ms',

    status: '连接异常'

  },

  {

    id: 5,

    name: 'svc-shipping',

    legend: 'svc-shipping',

    filterType: '单向',

    serviceGroup: '自定义服务组',

    metrics: '分组聚合详情>150ms',

    status: '正常'

  }

])



// 数据库表详情

const pageSize = ref(10)

const currentPage = ref(1)

const total = ref(5)



// 服务弹窗交互逻辑

const serviceDialogVisible = ref(false)

const deleteDialogVisible = ref(false)

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

  name: [{ required: true, message: '请输入服务名称', trigger: 'blur' }],

  legend: [{ required: true, message: '请输入图例', trigger: 'blur' }]

})



const selectedServiceId = ref(0)



// 表单验证

const isServiceFormValid = computed(() => {

  return serviceForm.value.name && serviceForm.value.legend

})



// 返回列表

const goBack = () => {

  router.push('/business')

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



// 确认删除操作

const confirmDeleteService = (id: number) => {

  selectedServiceId.value = id

  deleteDialogVisible.value = true

}



// 删除服务

const deleteService = () => {

  servicesData.value = servicesData.value.filter(item => item.id !== selectedServiceId.value)

  total.value = servicesData.value.length

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

 // 新建/编辑

    const newService = {

      id: servicesData.value.length + 1,

      name: serviceForm.value.name,

      legend: serviceForm.value.legend,

      filterType: serviceForm.value.filterType,

      serviceGroup: serviceForm.value.serviceGroup,

      metrics: serviceForm.value.metrics,

      status: '正常'

    }

    servicesData.value.push(newService)

    total.value++

  }

  serviceDialogVisible.value = false

  }



// 数据库表变化

const handlePageChange = (page: number) => {

  currentPage.value = page

  }

</script>



<style scoped>

.business-services-view {

  padding: 24px;

}



.services-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 24px;

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

  background-color: white;

  border-radius: 4px;

  padding: 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.services-content .el-table {

  margin-bottom: 24px;

}



.services-content .el-table th {

  background-color: #f5f7fa;

}



.services-content .el-table td {

  padding: 12px;

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

  margin-top: 24px;

}



.dialog-footer {

  display: flex;

  justify-content: flex-end;

  gap: 10px;

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



:deep(.el-button--primary) {

  background-color: #1677FF;

  border-color: #1677FF;

}



:deep(.el-button--danger) {

  background-color: #FF4D4F;

  border-color: #FF4D4F;

}

</style>