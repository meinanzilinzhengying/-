<template>

  <div class="variable-template">

    <div class="page-header">

      <el-breadcrumb separator="/">

        <el-breadcrumb-item><router-link to="/views/list">视频列表</router-link></el-breadcrumb-item>

        <el-breadcrumb-item>变量模板管理</el-breadcrumb-item>

      </el-breadcrumb>

      <h2>变量模板管理</h2>

      <el-button type="primary" @click="createTemplate">

        新建变量模板

      </el-button>

    </div>

    

    <div class="search-section">

      <el-form :inline="true" :model="searchForm" class="demo-form-inline">

        <el-form-item label="时间范围">

          <el-input v-model="searchForm.keyword" placeholder="输入模板策略" style="width: 200px;"></el-input>

        </el-form-item>

        <el-form-item>

          <el-button type="primary" @click="search">搜索</el-button>

          <el-button @click="reset">

            显示

          </el-button>

        </el-form-item>

      </el-form>

    </div>

    

    <div class="table-section">

      <el-table :data="templateList" style="width: 100%">

        <el-table-column prop="name" label="模板策略" width="200">

          <template #default="scope">

            <span class="template-name">{{ scope.row.name }}</span>

          </template>

        </el-table-column>

        <el-table-column prop="description" label="响应时间" />

        <el-table-column prop="variableType" label="变量类型" width="120">

          <template #default="scope">

            <el-tag :type="getTypeTagType(scope.row.variableType)">

              {{ scope.row.variableType }}

            </el-tag>

          </template>

        </el-table-column>

        <el-table-column prop="createTime" label="创建时间" width="180" />

        <el-table-column prop="updateTime" label="更新时间" width="180" />

        <el-table-column prop="creator" label="创建人" width="120" />

        <el-table-column label="操作" width="180" fixed="right">

          <template #default="scope">

            <el-button size="small" @click="viewTemplate(scope.row.id)">

              查看

            </el-button>

            <el-button size="small" type="primary" @click="editTemplate(scope.row.id)">

              编辑

            </el-button>

            <el-button size="small" type="danger" @click="deleteTemplate(scope.row.id)">

              分组聚合网络

            </el-button>

          </template>

        </el-table-column>

      </el-table>

      

      <div class="pagination">

        <el-pagination

          @size-change="handleSizeChange"

          @current-change="handleCurrentChange"

          :current-page="pagination.current"

          :page-sizes="[10, 20, 50, 100]"

          :page-size="pagination.size"

          layout="total, sizes, prev, pager, next, jumper"

          :total="pagination.total"

        />

      </div>

    </div>

    

 <!-- 新建/编辑变量模板对话框 -->

    <el-dialog

      v-model="dialogVisible"

      :title="dialogTitle"

      width="600px"

    >

      <el-form :model="templateForm" :rules="templateRules" ref="templateFormRef">

        <el-form-item label="模板策略" prop="name">

          <el-input v-model="templateForm.name" placeholder="请输入模板名称" />

        </el-form-item>

        <el-form-item label="响应时间" prop="description">

          <el-input

            v-model="templateForm.description"

            type="textarea"

            placeholder="请输入变量描述"

            :rows="3"

          ></el-input>

        </el-form-item>

        <el-form-item label="变量类型" prop="variableType">

          <el-select v-model="templateForm.variableType" placeholder="请选择变量类型" @change="handleVariableTypeChange">

            <el-option label="文本输入类型" value="text" />

            <el-option label="分组依据" value="group" />

            <el-option label="服务" value="service" />

            <el-option label="服务分组" value="service_group" />

            <el-option label="区域" value="region" />

            <el-option label="主机名" value="host" />

            <el-option label="容器" value="container" />

          </el-select>

        </el-form-item>

        

 <!-- 文本输入类型 -->

        <el-form-item v-if="templateForm.variableType === 'text'" label="变量值" prop="variableValue">

          <el-input

            v-model="templateForm.variableValue"

            type="textarea"

            placeholder="请输入延迟分布，多个数值用逗号间隔"

            :rows="3"

          ></el-input>

        </el-form-item>

        

 <!-- 分组类型 -->

        <template v-if="templateForm.variableType === 'group'">

          <el-form-item label="数据库表" prop="dataTable">

            <el-select v-model="templateForm.dataTable" placeholder="请选择数据源和拓扑图">

              <el-option label="flow_log" value="flow_log" />

              <el-option label="service_metrics" value="service_metrics" />

              <el-option label="host_metrics" value="host_metrics" />

              <el-option label="container_metrics" value="container_metrics" />

            </el-select>

          </el-form-item>

          <el-form-item label="选择Tag" prop="tag">

            <el-select v-model="templateForm.tag" placeholder="请选择Tag">

              <el-option label="pod_ns" value="pod_ns" />

              <el-option label="service" value="service" />

              <el-option label="service_group" value="service_group" />

              <el-option label="region" value="region" />

              <el-option label="host" value="host" />

              <el-option label="container" value="container" />

            </el-select>

          </el-form-item>

        </template>

        

 <!-- 其他类型 -->

        <el-form-item v-else label="变量值" prop="variableValue">

          <el-input

            v-model="templateForm.variableValue"

            type="textarea"

            placeholder="请输入延迟分布，多个数值用逗号间隔"

            :rows="3"

          ></el-input>

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="dialogVisible = false">取消</el-button>

          <el-button type="primary" @click="submitForm">确定</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 删除确认对话框 -->

    <el-dialog

      v-model="deleteDialogVisible"

      title="删除确认"

      width="400px"

    >

      <p>确定要删除这个变量模板吗？</p>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="deleteDialogVisible = false">取消</el-button>

          <el-button type="danger" @click="confirmDelete">确认删除</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive } from 'vue'



// 搜索处理函数

const searchForm = reactive({

  keyword: ''

})



// 数据库表

const pagination = reactive({

  current: 1,

  size: 10,

  total: 100

})



// 变量模板列表

const templateList = ref([

  {

    id: 1,

    name: '文本输入框',

    description: '文本输入类型的变量模板示例',

    variableType: 'text',

    variableValue: 'value1,value2,value3',

    createTime: '2023-09-01 10:00:00',

    updateTime: '2023-09-01 10:00:00',

    creator: 'admin'

  },

  {

    id: 2,

    name: '下拉选择框',

    description: '分组类型的变量模板示例',

    variableType: 'group',

    dataTable: 'service_metrics',

    tag: 'pod_ns',

    createTime: '2023-09-01 11:00:00',

    updateTime: '2023-09-01 11:00:00',

    creator: 'admin'

  },

  {

    id: 3,

    name: '服务列表',

    description: '所有已选标签的列表',

    variableType: 'service',

    variableValue: 'web-shop,svc-user,svc-order,svc-payment,svc-shipping',

    createTime: '2023-09-01 12:00:00',

    updateTime: '2023-09-01 12:00:00',

    creator: 'admin'

  },

  {

    id: 4,

    name: '服务列表',

    description: '所有已选标签组的列表',

    variableType: 'service_group',

    variableValue: '服务分组,服务分组,服务分组',

    createTime: '2023-09-01 13:00:00',

    updateTime: '2023-09-01 13:00:00',

    creator: 'admin'

  },

  {

    id: 5,

    name: '区域列表',

    description: '所有区域的列表',

    variableType: 'region',

    variableValue: '区域1,区域2,区域3',

    createTime: '2023-09-01 14:00:00',

    updateTime: '2023-09-01 14:00:00',

    creator: 'admin'

  },

  {

    id: 6,

    name: '主机列表',

    description: '所有主机的列表',

    variableType: 'host',

    variableValue: '主机名1,主机名2,主机名3',

    createTime: '2023-09-01 15:00:00',

    updateTime: '2023-09-01 15:00:00',

    creator: 'admin'

  },

  {

    id: 7,

    name: '容器列表',

    description: '所有进程的列表',

    variableType: 'container',

    variableValue: '容器1,容器2,容器3',

    createTime: '2023-09-01 16:00:00',

    updateTime: '2023-09-01 16:00:00',

    creator: 'admin'

  }

])



// 对话框

const dialogVisible = ref(false)

const deleteDialogVisible = ref(false)

const dialogTitle = ref('新建变量模板')

const currentTemplateId = ref(0)



// 表单

const templateForm = reactive({

  name: '',

  description: '',

  variableType: '',

  variableValue: '',

  dataTable: '',

  tag: ''

})



const templateFormRef = ref()



// 表单验证规则

const templateRules = reactive({

  name: [

    { required: true, message: '请输入模板名称', trigger: 'blur' },

    { min: 1, max: 50, message: '长度在 1 到 50 个字符', trigger: 'blur' }

  ],

  description: [

    { max: 200, message: '长度不能超过 200 个字符', trigger: 'blur' }

  ],

  variableType: [

    { required: true, message: '请选择变量类型', trigger: 'change' }

  ],

  variableValue: [

    { required: true, message: '请输入变量名称', trigger: 'blur' }

  ],

  dataTable: [

    { required: true, message: '请选择数据源和拓扑图', trigger: 'change' }

  ],

  tag: [

    { required: true, message: '请选择Tag', trigger: 'change' }

  ]

})



// 处理变量类型变化

const handleVariableTypeChange = () => {

  if (templateForm.variableType === 'group') {

    templateForm.dataTable = ''

    templateForm.tag = ''

  } else {

    templateForm.dataTable = ''

    templateForm.tag = ''

  }

}



// 获取类型标签数据

const getTypeTagType = (type: string) => {

  switch (type) {

    case 'text':

      return 'primary'

    case 'group':

      return 'success'

    case 'service':

      return 'warning'

    case 'service_group':

      return 'info'

    case 'region':

      return 'danger'

    case 'host':

      return 'primary'

    case 'container':

      return 'success'

    default:

      return ''

  }

}



// 最近5分钟

const search = () => {

 // 实现搜索过滤功能

}



// 显示

const reset = () => {

  searchForm.keyword = ''

 // // 处理相关事件

}



// 数据库表

const handleSizeChange = (size: number) => {

  pagination.size = size

 // // 确认相关事件

}



const handleCurrentChange = (current: number) => {

  pagination.current = current

 // // 确认相关事件

}



// 新建变量模板

const createTemplate = () => {

  dialogTitle.value = '新建变量模板'

  currentTemplateId.value = 0

  templateForm.name = ''

  templateForm.description = ''

  templateForm.variableType = ''

  templateForm.variableValue = ''

  dialogVisible.value = true

}



// 编辑变量模板

const editTemplate = (id: number) => {

  dialogTitle.value = '编辑变量模板'

  currentTemplateId.value = id

 // 模拟获取指标模板数据流

  const template = templateList.value.find(t => t.id === id)

  if (template) {

    templateForm.name = template.name

    templateForm.description = template.description

    templateForm.variableType = template.variableType

    templateForm.variableValue = template.variableValue || ''

    templateForm.dataTable = template.dataTable || ''

    templateForm.tag = template.tag || ''

  }

  dialogVisible.value = true

}



// 删除变量模板

const deleteTemplate = (id: number) => {

  currentTemplateId.value = id

  deleteDialogVisible.value = true

}



// 确认删除操作

const confirmDelete = () => {

 // // 删除相关事件

  deleteDialogVisible.value = false

}



// 查看变量模板

const viewTemplate = (id: number) => {

 // 解析并处理可观测性

}



// 重置搜索条件

const submitForm = async () => {

  if (!templateFormRef.value) return

  try {

    await templateFormRef.value.validate()

 // 实现提交保存功能

    dialogVisible.value = false

  } catch (e) {

    console.error('表单验证失败', e)

  }

}

</script>



<style scoped>

.variable-template {

  background-color: white;

  border-radius: 4px;

  padding: 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  height: 100%;

  display: flex;

  flex-direction: column;

  gap: 24px;

}



.page-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  padding-bottom: 16px;

  border-bottom: 1px solid #e4e7ed;

}



.page-header h2 {

  margin: 0;

  font-size: 18px;

  font-weight: bold;

  color: #303133;

}



.search-section {

  padding: 16px 0;

  border-bottom: 1px solid #e4e7ed;

}



.table-section {

  flex: 1;

  overflow: auto;

}



.template-name {

  font-weight: bold;

  color: #303133;

}



.pagination {

  padding-top: 16px;

  border-top: 1px solid #e4e7ed;

  display: flex;

  justify-content: flex-end;

}



.dialog-footer {

  display: flex;

  justify-content: flex-end;

  gap: 10px;

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