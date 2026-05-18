<template>

  <div class="chart-settings">

    <el-dropdown trigger="click">

      <el-button >

        设置 <el-icon class="el-icon--right"><ArrowDown /></el-icon>

      </el-button>

      <template #dropdown>

        <el-dropdown-menu>

          <el-dropdown-item @click="editChart">编辑</el-dropdown-item>

          <el-dropdown-item @click="addToView">应用到视图</el-dropdown-item>

          <el-dropdown-item @click="downloadCSV">下载CSV</el-dropdown-item>

          <el-dropdown-item @click="viewAPI">查看API</el-dropdown-item>

          <el-dropdown-item @click="switchChartType">切换图表类型</el-dropdown-item>

        </el-dropdown-menu>

      </template>

    </el-dropdown>

    

 <!-- 添加子图窗口-->

    <el-dialog

      v-model="addToViewVisible"

      title="应用到视图"

      width="600px"

    >

      <el-form :model="addForm" :rules="addRules" ref="addFormRef">

        <el-form-item label="图表策略" prop="name">

          <el-input v-model="addForm.name" placeholder="请输入图表名称" />

        </el-form-item>

        <el-form-item label="数据库源" prop="dataSource">

          <el-input v-model="addForm.dataSource" placeholder="数据库源" readonly></el-input>

        </el-form-item>

        <el-form-item label="视图" prop="viewId">

          <el-select v-model="addForm.viewId" placeholder="请选择视图" style="width: 100%;">

            <el-option 

              v-for="view in views" 

              :key="view.id" 

              :label="view.name" 

              :value="view.id"

              :disabled="view.type === 'built-in'"

            />

          </el-select>

          <div class="view-actions">

            <el-link type="primary" @click="showCreateViewDialog">新建视图</el-link>

          </div>

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="addToViewVisible = false">取消</el-button>

          <el-button type="primary" @click="submitAddToView" :disabled="!addForm.viewId">确定</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 新建视频窗口 -->

    <el-dialog

      v-model="createViewVisible"

      title="新建视图"

      width="600px"

    >

      <el-form :model="createForm" :rules="createRules" ref="createFormRef">

        <el-form-item label="视图名称" prop="name">

          <el-input v-model="createForm.name" placeholder="请输入视图名称" />

        </el-form-item>

        <el-form-item label="响应时间" prop="description">

          <el-input

            v-model="createForm.description"

            type="textarea"

            placeholder="请输入描述信息"

            :rows="3"

          ></el-input>

        </el-form-item>

        <el-form-item label="团队" prop="team">

          <el-select v-model="createForm.team" placeholder="请选择团队">

            <el-option label="团队A" value="teamA" />

            <el-option label="团队B" value="teamB" />

            <el-option label="团队C" value="teamC" />

          </el-select>

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="createViewVisible = false">取消</el-button>

          <el-button type="primary" @click="submitCreateView">确定</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive } from 'vue'

import { ArrowDown } from '@element-plus/icons-vue'



const props = defineProps({

  chartName: {

    type: String,

    default: '图表'

  },

  dataSource: {

    type: String,

    default: 'DeepFlow'

  }

})



const emit = defineEmits(['edit', 'add', 'download', 'api', 'switch-type'])



// 视频列表

const views = ref([

  { id: 1, name: '系统概览', type: 'custom' },

  { id: 2, name: '默认视图', type: 'custom' },

  { id: 3, name: '系统性能监控', type: 'built-in' },

  { id: 4, name: '数据流监控', type: 'custom' }

])



// 添加子图窗口
const addToViewVisible = ref(false)

const addForm = reactive({

  name: props.chartName,

  dataSource: props.dataSource,

  viewId: ''

})



const addFormRef = ref()



// 应用拓扑模块-网络设备
const addRules = reactive({

  name: [

    { required: true, message: '请输入图表名称', trigger: 'blur' },

    { min: 1, max: 50, message: '长度应在 1 到 50 个字符', trigger: 'blur' }

  ],

  dataSource: [

    { required: true, message: '请输入图表描述', trigger: 'blur' }

  ],

  viewId: [

    { required: true, message: '请选择视图', trigger: 'change' }

  ]

})



// 新建视频窗口

const createViewVisible = ref(false)

const createForm = reactive({

  name: '',

  description: '',

  team: ''

})



const createFormRef = ref()



// 新建连接网络模块-可用区域

const createRules = reactive({

  name: [

    { required: true, message: '请输入图表名称', trigger: 'blur' },

    { min: 1, max: 50, message: '长度应在 1 到 50 个字符', trigger: 'blur' }

  ],

  team: [

    { required: true, message: '请选择团队', trigger: 'change' }

  ]

})



// 编辑图表

const editChart = () => {

  emit('edit')

}



// 应用拓扑
const addToView = () => {

  addToViewVisible.value = true

}



// 下载CSV

const downloadCSV = () => {

  emit('download')

}



// 查看API

const viewAPI = () => {

  emit('api')

}



// 切换图表类型

const switchChartType = () => {

  emit('switch-type')

}



// 显示新建视频窗口

const showCreateViewDialog = () => {

  createViewVisible.value = true

}



// 提交新建视图

const submitCreateView = async () => {

  if (!createFormRef.value) return

  try {

    await createFormRef.value.validate()

 // 模拟创建告警策略

    const newView = {

      id: views.value.length + 1,

      name: createForm.name,

      type: 'custom'

    }

    views.value.push(newView)

    addForm.viewId = newView.id

    createViewVisible.value = false

  } catch (e) {

    console.error('新建连接网络模块-验证失败', e)

  }

}



// 提交应用到视图

const submitAddToView = async () => {

  if (!addFormRef.value) return

  try {

    await addFormRef.value.validate()

 // 模拟应用到视图

    emit('add', addForm)

    addToViewVisible.value = false

  } catch (e) {

    console.error('应用拓扑模块-网络设备管理页面', e)

  }

}

</script>



<style scoped>

.chart-settings {

  display: inline-block;

}



.view-actions {

  margin-top: 8px;

  text-align: right;

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