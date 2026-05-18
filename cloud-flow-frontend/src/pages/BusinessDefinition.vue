<template>

  <div class="business-definition">

    <div class="definition-header">

      <h3>业务定义</h3>

      <div class="header-actions">

        <el-form :inline="true" :model="searchForm" class="demo-form-inline">

          <el-form-item>

            <el-input v-model="searchForm.keyword" placeholder="搜索业务" style="width: 200px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchBusiness">搜索</el-button>

          </el-form-item>

        </el-form>

        <el-button type="primary" @click="createBusiness" style="margin-left: 10px;">

          新建业务

        </el-button>

      </div>

    </div>

    <div class="definition-content">

      <el-table :data="businessData" style="width: 100%">

        <el-table-column prop="starred" label="星标" width="60">

          <template #default="scope">

            <el-button

              @click="toggleStar(scope.row)"

              :class="{ 'starred': scope.row.starred }"

            >

              <component :is="scope.row.starred ? StarFilled : Star" />

            </el-button>

          </template>

        </el-table-column>

        <el-table-column prop="name" label="名称" width="180" sortable />

        <el-table-column prop="team" label="团队" width="120" />

        <el-table-column prop="table" label="数据库表" width="150" />

        <el-table-column prop="services" label="服务" width="150" />

        <el-table-column prop="serviceGroups" label="服务组" width="150" />

        <el-table-column prop="paths" label="路径" width="150" />

        <el-table-column prop="creator" label="创建者" width="100" />

        <el-table-column prop="updateTime" label="更新时间" width="180" />

        <el-table-column label="操作" width="250" fixed="right">

          <template #default="scope">

            <el-button size="small" @click="viewBusiness(scope.row)">

              查看拓扑

            </el-button>

            <el-button size="small" @click="viewTopology(scope.row)">

              请求链路追踪

            </el-button>

            <el-button size="small" @click="viewServiceList(scope.row)">

              服务列表

            </el-button>

            <el-button size="small" @click="editBusiness(scope.row)">

              编辑

            </el-button>

            <el-button size="small" type="danger" @click="confirmDelete(scope.row.id)">

              删除

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

    

 <!-- 新建连接/编辑连接对话框 -->

    <el-dialog

      v-model="dialogVisible"

      :title="dialogTitle"

      width="600px"

    >

      <el-form :model="businessForm" label-width="100px" :rules="businessRules" ref="businessFormRef">

        <el-form-item label="业务策略" prop="name" required>

          <el-input v-model="businessForm.name" placeholder="请输入业务名称" />

        </el-form-item>

        <el-form-item label="团队" prop="team">

          <el-select v-model="businessForm.team" placeholder="请选择团队" style="width: 100%">

            <el-option label="团队A" value="teamA" />

            <el-option label="团队B" value="teamB" />

            <el-option label="团队C" value="teamC" />

          </el-select>

        </el-form-item>

        <el-form-item label="数据库表" prop="table" required>

          <el-select v-model="businessForm.table" placeholder="请选择数据库表" style="width: 100%">

            <el-option label="表1" value="table1" />

            <el-option label="表2" value="table2" />

            <el-option label="表3" value="table3" />

          </el-select>

        </el-form-item>

        <el-form-item label="指标" prop="metrics">

          <el-select

            v-model="businessForm.metrics"

            multiple

            placeholder="请选择指标"

            style="width: 100%"

            :disabled="businessForm.metrics.length >= 10"

          >

            <el-option label="网络流量监控" value="request_rate" />

            <el-option label="分组聚合详情" value="response_time" />

            <el-option label="错误率" value="error_rate" />

            <el-option label="QPS" value="qps" />

            <el-option label="流量" value="traffic" />

            <el-option label="延迟" value="latency" />

            <el-option label="丢包率" value="packet_loss" />

            <el-option label="CPU使用率" value="cpu_usage" />

            <el-option label="内存使用率" value="mem_usage" />

            <el-option label="磁盘使用率" value="disk_usage" />

            <el-option label="网络使用率" value="network_usage" />

          </el-select>

          <div class="metric-hint" v-if="businessForm.metrics.length >= 10">

            最多选择10个指标

          </div>

        </el-form-item>

        <el-form-item label="响应时间">

          <el-input

            v-model="businessForm.description"

            type="textarea"

            :rows="3"

            placeholder="请输入服务名称或网络地址"

          />

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="dialogVisible = false">取消</el-button>

          <el-button type="primary" @click="saveBusiness" :disabled="!isBusinessFormValid">保存</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 删除确认对话框-->

    <el-dialog

      v-model="deleteDialogVisible"

      title="确认删除"

      width="400px"

    >

      <p>确认要删除此业务吗？</p>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="deleteDialogVisible = false">取消</el-button>

          <el-button type="danger" @click="deleteBusiness">删除</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

import { ref, computed, onMounted } from 'vue'

import { Star, StarFilled } from '@element-plus/icons-vue'

import { useRouter } from 'vue-router'

import { useBusinessStore } from '../stores'

import { api } from '../utils/api'



const router = useRouter()

const businessStore = useBusinessStore()



onMounted(async () => {

  await loadBusinessData()

})



const loadBusinessData = async () => {

  try {

    await businessStore.fetchBusinesses()

    businessData.value = businessStore.businesses.map(b => ({

      id: b.id,

      name: b.name,

      team: b.team || '',

      table: b.table || '',

      services: b.services?.length?.toString() || '0',

      serviceGroups: b.serviceGroups?.length?.toString() || '0',

      paths: b.paths || '0',

      creator: b.creator || 'admin',

      updateTime: b.updatedAt || b.createdAt || '',

      starred: false

    }))

    total.value = businessData.value.length

  } catch (e) {

    console.error('获取业务服务数据失败:', e)

  }

}



// 搜索表单

const searchForm = ref({

  keyword: ''

})



// 网络分析模块数据

const businessData = ref([

  {

    id: 1,

    name: '电商业务',

    team: '团队A',

    table: 'table1',

    services: '5',

    serviceGroups: '2',

    paths: '3',

    creator: 'admin',

    updateTime: '2023-09-01 10:00:00',

    starred: true

  },

  {

    id: 2,

    name: '物流业务',

    team: '团队B',

    table: 'table2',

    services: '3',

    serviceGroups: '1',

    paths: '2',

    creator: 'admin',

    updateTime: '2023-09-02 14:30:00',

    starred: false

  },

  {

    id: 3,

    name: '营销业务',

    team: '团队C',

    table: 'table3',

    services: '4',

    serviceGroups: '2',

    paths: '1',

    creator: 'admin',

    updateTime: '2023-09-03 09:15:00',

    starred: false

  }

])



// 分析结果

const pageSize = ref(10)

const currentPage = ref(1)

const total = ref(3)



// 对话框状态

const dialogVisible = ref(false)

const deleteDialogVisible = ref(false)

const dialogTitle = ref('新建业务')

const businessFormRef = ref()

const businessForm = ref({

  id: '',

  name: '',

  team: '',

  table: '',

  metrics: [],

  description: ''

})



const businessRules = ref({

  name: [{ required: true, message: '请输入业务名称', trigger: 'blur' }],

  table: [{ required: true, message: '请选择数据库表', trigger: 'change' }]

})



const selectedBusinessId = ref(0)



// 表单验证

const isBusinessFormValid = computed(() => {

  return businessForm.value.name && businessForm.value.table

})



// 搜索业务

const searchBusiness = () => {

 // 模拟数据

}



// 设置对话框触发

const toggleStar = (row: any) => {

  row.starred = !row.starred

 // 按星标排序编辑器

  businessData.value.sort((a, b) => {

    if (a.starred && !b.starred) return -1

    if (!a.starred && b.starred) return 1

    return b.name.localeCompare(a.name)

  })

  }



// 查看业务详情

const viewBusiness = (row: any) => {

  router.push(`/business/detail/${row.id}`)

  }



// 查看业务拓扑

const viewTopology = (row: any) => {

  router.push(`/business/topology/${row.id}`)

  }



// 查看服务列表

const viewServiceList = (row: any) => {

  router.push(`/business/services/${row.id}`)

  }



// 创建业务

const createBusiness = () => {

  dialogTitle.value = '新建业务'

  businessForm.value = {

    id: '',

    name: '',

    team: '',

    table: '',

    metrics: [],

    description: ''

  }

  dialogVisible.value = true

}



// 编辑连接管理

const editBusiness = (row: any) => {

  dialogTitle.value = '编辑连接管理'

  businessForm.value = {

    id: row.id,

    name: row.name,

    team: row.team,

    table: row.table,

    metrics: [],

    description: ''

  }

  dialogVisible.value = true

}



// 确认删除

const confirmDelete = (id: number) => {

  selectedBusinessId.value = id

  deleteDialogVisible.value = true

}



// 删除连接

const deleteBusiness = async () => {

  try {

    await businessStore.deleteBusiness(selectedBusinessId.value.toString())

    businessData.value = businessData.value.filter(item => item.id !== selectedBusinessId.value)

    total.value = businessData.value.length

  } catch (e) {

    console.error('删除连接失败:', e)

  }

  deleteDialogVisible.value = false

}



// 保存业务

const saveBusiness = async () => {

  try {

    if (businessForm.value.id) {

 // 更新连接

      await businessStore.updateBusiness(businessForm.value.id, {

        name: businessForm.value.name,

        team: businessForm.value.team,

        table: businessForm.value.table,

        description: businessForm.value.description

      })

    } else {

 // 新建编辑

      await businessStore.createBusiness({

        name: businessForm.value.name,

        team: businessForm.value.team,

        table: businessForm.value.table,

        description: businessForm.value.description

      })

    }

    await loadBusinessData()

    dialogVisible.value = false

  } catch (e) {

    console.error('保存业务失败:', e)

  }

}



// 分页变化

const handlePageChange = (page: number) => {

  currentPage.value = page

  }

</script>



<style scoped>

.business-definition {

  padding: 24px;

}



.definition-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 24px;

}



.definition-header h3 {

  margin: 0;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.header-actions {

  display: flex;

  gap: 10px;

}



.definition-content {

  background-color: #f5f7fa;

  border-radius: 4px;

  padding: 24px;

}



.definition-content .el-table {

  margin-bottom: 24px;

}



.definition-content .el-table th {

  background-color: #f5f7fa;

}



.definition-content .el-table td {

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



.starred {

  color: #1677FF;

}



.metric-hint {

  font-size: 12px;

  color: #909399;

  margin-top: 5px;

}



@media (max-width: 1200px) {

  .definition-header {

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