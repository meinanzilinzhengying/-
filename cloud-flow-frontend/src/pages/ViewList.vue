<template>

  <div class="view-list">

 <!-- 顶部标签 -->

    <div class="view-tabs">

      <el-tabs v-model="activeTab" class="tab-container">

        <el-tab-pane label="视图" name="all" />

        <el-tab-pane label="自定义视图" name="custom" />

        <el-tab-pane label="内置视图" name="built-in" />

      </el-tabs>

    </div>

    

 <!-- 内置视图列表-->

    <div class="action-bar">

      <div class="action-left">

        <el-button type="primary" @click="createView">

          新建视图

        </el-button>

        <el-button @click="importView">

          导入视图

        </el-button>

      </div>

      <div class="action-right">

        <el-input v-model="searchForm.keyword" placeholder="搜索视图" style="width: 200px; margin-right: 10px;"></el-input>

        <el-button @click="showColumnSettings">

          列设置

        </el-button>

        <el-button @click="exportSelected" :disabled="selectedViews.length === 0">

          批量导出

        </el-button>

        <el-button type="danger" @click="deleteSelected" :disabled="selectedViews.length === 0">

          批量删除

        </el-button>

      </div>

    </div>

    

 <!-- 表格 -->

    <div class="table-section">

      <el-table

        :data="filteredViewList"

        style="width: 100%"

        header-cell-class-name="fixed-header"

        @selection-change="handleSelectionChange"

      >

        <el-table-column type="selection" width="50" />

        <el-table-column prop="name" label="名称" width="200" sortable>

          <template #default="scope">

            <div class="view-name" :class="{ 'built-in': scope.row.type === 'built-in' }">

              {{ scope.row.name }}

            </div>

          </template>

        </el-table-column>

        <el-table-column prop="team" label="团队" width="120" sortable />

        <el-table-column prop="description" label="响应时间" sortable />

        <el-table-column prop="bindDrawer" label="绑定抽屉" width="120" sortable>

          <template #default="scope">

            <el-switch v-model="scope.row.bindDrawer" disabled />

          </template>

        </el-table-column>

        <el-table-column prop="creator" label="创建人" width="120" sortable />

        <el-table-column prop="updateTime" label="最近修改时间" width="180" sortable />

        <el-table-column label="操作" width="200" fixed="right">

          <template #default="scope">

            <el-button size="small" @click="viewDetail(scope.row.id)">

              查看

            </el-button>

            <el-button 

              size="small" 

              type="primary" 

              @click="editView(scope.row.id)"

              :disabled="scope.row.type === 'built-in'"

              :class="{ 'disabled-button': scope.row.type === 'built-in' }"

            >

              编辑

            </el-button>

            <el-button 

              size="small" 

              @click="exportView(scope.row.id)"

            >

              导出数据库

            </el-button>

            <el-button 

              size="small" 

              type="danger" 

              @click="deleteView(scope.row.id)"

              :disabled="scope.row.type === 'built-in'"

              :class="{ 'disabled-button': scope.row.type === 'built-in' }"

            >

              分组聚合网络

            </el-button>

            <el-button 

              size="small" 

              @click="toggleStar(scope.row)"

              :icon="scope.row.starred ? 'StarFilled' : 'Star'"

              :class="{ 'starred': scope.row.starred }"

            />

          </template>

        </el-table-column>

      </el-table>

      

 <!-- 数据库表 -->

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

    

 <!-- 新建/编辑/编辑/导入视频对话框-->

    <el-dialog

      v-model="dialogVisible"

      :title="dialogTitle"

      width="600px"

    >

      <el-form :model="viewForm" :rules="viewRules" ref="viewFormRef">

        <el-form-item label="视图名称" prop="name">

          <el-input v-model="viewForm.name" placeholder="请输入视图名称" />

        </el-form-item>

        <el-form-item label="团队" prop="team">

          <el-select v-model="viewForm.team" placeholder="请选择团队">

            <el-option label="团队A" value="teamA" />

            <el-option label="团队B" value="teamB" />

            <el-option label="团队C" value="teamC" />

          </el-select>

        </el-form-item>

        <el-form-item label="响应时间" prop="description">

          <el-input

            v-model="viewForm.description"

            type="textarea"

            placeholder="请输入描述信息"

            :rows="3"

          ></el-input>

        </el-form-item>

        <el-form-item label="绑定抽屉" prop="bindDrawer">

          <el-switch v-model="viewForm.bindDrawer" />

        </el-form-item>

        <el-form-item label="数据库来源" prop="dataSource">

          <el-select v-model="viewForm.dataSource" placeholder="请选择数据源">

            <el-option label="Prometheus" value="prometheus" />

            <el-option label="Grafana" value="grafana" />

            <el-option label="DeepFlow" value="deepflow" />

          </el-select>

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="dialogVisible = false">取消</el-button>

          <el-button type="primary" @click="submitForm">确定</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 列设置对话框-->

    <el-dialog

      v-model="columnSettingsVisible"

      title="列设置"

      width="400px"

    >

      <div class="column-settings">

        <el-button @click="distributeColumns('equal')">

          平均列表

        </el-button>

        <el-button @click="distributeColumns('content')">

          按内容搜索

        </el-button>

      </div>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="columnSettingsVisible = false">关闭</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 删除确认对话框 -->

    <el-dialog

      v-model="deleteDialogVisible"

      title="删除确认"

      width="400px"

    >

      <p>{{ deleteMessage }}</p>

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

import { ref, reactive, computed } from 'vue'

import { useRouter } from 'vue-router'

import { Star, StarFilled } from '@element-plus/icons-vue'



const router = useRouter()



// 顶部标签

const activeTab = ref('custom')



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



// 视频列表

const viewList = ref([

  {

    id: 1,

    name: '系统概览',

    team: '团队A',

    description: '系统整体运行状态',

    bindDrawer: true,

    createTime: '2023-09-01 10:00:00',

    updateTime: '2023-09-01 10:00:00',

    creator: 'admin',

    type: 'built-in',

    starred: true

  },

  {

    id: 2,

    name: '默认视图',

    team: '团队A',

    description: '服务运行状态监控',

    bindDrawer: true,

    createTime: '2023-09-01 11:00:00',

    updateTime: '2023-09-01 11:00:00',

    creator: 'admin',

    type: 'custom',

    starred: false

  },

  {

    id: 3,

    name: '系统性能监控',

    team: '团队B',

    description: '网络运行状态监控',

    bindDrawer: false,

    createTime: '2023-09-01 12:00:00',

    updateTime: '2023-09-01 12:00:00',

    creator: 'admin',

    type: 'custom',

    starred: true

  },

  {

    id: 4,

    name: '数据流监控',

    team: '团队B',

    description: '数据库服务运行状态和创建人',

    bindDrawer: false,

    createTime: '2023-09-01 13:00:00',

    updateTime: '2023-09-01 13:00:00',

    creator: 'admin',

    type: 'custom',

    starred: false

  },

  {

    id: 5,

    name: '应用最终指标',

    team: '团队C',

    description: '应用运行状态和创建人',

    bindDrawer: true,

    createTime: '2023-09-01 14:00:00',

    updateTime: '2023-09-01 14:00:00',

    creator: 'admin',

    type: 'custom',

    starred: false

  },

  {

    id: 6,

    name: '安全视图',

    team: '团队C',

    description: '系统安全监控状态',

    bindDrawer: true,

    createTime: '2023-09-01 15:00:00',

    updateTime: '2023-09-01 15:00:00',

    creator: 'admin',

    type: 'built-in',

    starred: false

  }

])



// 选中的视图

const selectedViews = ref<any[]>([])



// 对话框

const dialogVisible = ref(false)

const deleteDialogVisible = ref(false)

const columnSettingsVisible = ref(false)

const dialogTitle = ref('新建视图')

const currentViewId = ref(0)

const deleteMessage = ref('确定要删除这个视图吗？')



// 表单

const viewForm = reactive({

  name: '',

  team: '',

  description: '',

  bindDrawer: false,

  dataSource: ''

})



const viewFormRef = ref()



// 表单验证规则

const viewRules = reactive({

  name: [

    { required: true, message: '请输入视图名称', trigger: 'blur' },
    { min: 1, max: 50, message: '长度在 1 到 50 个字符', trigger: 'blur' }

  ],

  team: [

    { required: true, message: '请选择团队', trigger: 'change' }

  ],

  description: [

    { max: 200, message: '长度不能超过200个字符', trigger: 'blur' }

  ],

  dataSource: [

    { required: true, message: '请选择数据源', trigger: 'change' }

  ]

})



// 过滤后的视图列表

const filteredViewList = computed(() => {

  let filtered = viewList.value

  

 // 按标签过滤

  if (activeTab.value === 'custom') {

    filtered = filtered.filter(view => view.type === 'custom')

  } else if (activeTab.value === 'built-in') {

    filtered = filtered.filter(view => view.type === 'built-in')

  }

  

 // 按关键词搜索过滤

  if (searchForm.keyword) {

    const keyword = searchForm.keyword.toLowerCase()

    filtered = filtered.filter(view => 

      view.name.toLowerCase().includes(keyword) ||

      view.team.toLowerCase().includes(keyword) ||

      view.description.toLowerCase().includes(keyword) ||

      view.creator.toLowerCase().includes(keyword) ||

      view.updateTime.toLowerCase().includes(keyword)

    )

  }

  

 // 按时间排序并支持分页
  filtered.sort((a, b) => {

    if (a.starred && !b.starred) return -1

    if (!a.starred && b.starred) return 1

    return 0

  })

  

  return filtered

})



// 最近5分钟

const search = () => {

 // 实现搜索功能

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



// 选择视图

const handleSelectionChange = (val: any[]) => {

  selectedViews.value = val

}



// 新建视图

const createView = () => {

  dialogTitle.value = '新建视图'

  currentViewId.value = 0

  viewForm.name = ''

  viewForm.team = ''

  viewForm.description = ''

  viewForm.bindDrawer = false

  viewForm.dataSource = ''

  dialogVisible.value = true

}



// 导入视图

const importView = () => {

  dialogTitle.value = '导入视图'

  currentViewId.value = 0

  viewForm.name = ''

  viewForm.team = ''

  viewForm.description = ''

  viewForm.bindDrawer = false

  viewForm.dataSource = ''

  dialogVisible.value = true

}



// 编辑视图

const editView = (id: number) => {

  dialogTitle.value = '编辑视图'

  currentViewId.value = id

 // 模板分页加载视图数据

  const view = viewList.value.find(v => v.id === id)

  if (view) {

    viewForm.name = view.name

    viewForm.team = view.team

    viewForm.description = view.description

    viewForm.bindDrawer = view.bindDrawer

    viewForm.dataSource = 'deepflow' // 模拟数据源

  }

  dialogVisible.value = true

}



// 删除视图

const deleteView = (id: number) => {

  currentViewId.value = id

  deleteMessage.value = '确定要删除这个视图吗？'

  deleteDialogVisible.value = true

}



// 批量删除

const deleteSelected = () => {

  if (selectedViews.value.length === 0) return

  deleteMessage.value = `确定要删除选中的 ${selectedViews.value.length} 个视图`

  deleteDialogVisible.value = true

}



// 确认删除操作

const confirmDelete = () => {

  if (selectedViews.value.length > 0) {

 // 批量删除

  } else {

 // 确认批量删除

    }

 // // 删除相关事件

  deleteDialogVisible.value = false

  selectedViews.value = []

}



// 查看视图模板详情

const viewDetail = (id: number) => {

  router.push(`/views/detail?id=${id}`)

}



// 导出视图

const exportView = (id: number) => {

 // 实现导出功能

}



// 批量导出

const exportSelected = () => {

  if (selectedViews.value.length === 0) return

 // 实现批量导出功能

}



// 设置对话框触发

const toggleStar = (view: any) => {

  view.starred = !view.starred

  }



// 显示列设置

const showColumnSettings = () => {

  columnSettingsVisible.value = true

}



// 配置列表

const distributeColumns = (type: string) => {

 // 实现列分配功能

  columnSettingsVisible.value = false

}



// 重置搜索条件

const submitForm = async () => {

  if (!viewFormRef.value) return

  try {

    await viewFormRef.value.validate()

 // 实现提交功能

    dialogVisible.value = false

  } catch (e) {

    console.error('表单验证失败', e)

  }

}

</script>



<style scoped>

.view-list {

  background-color: white;

  border-radius: 4px;

  padding: 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  height: 100%;

  display: flex;

  flex-direction: column;

  gap: 24px;

}



.view-tabs {

  margin-bottom: 16px;

}



.tab-container {

  border-bottom: 1px solid #e4e7ed;

}



.action-bar {

  display: flex;

  justify-content: space-between;

  align-items: center;

  padding: 16px 0;

  border-bottom: 1px solid #e4e7ed;

}



.action-left {

  display: flex;

  gap: 10px;

}



.action-right {

  display: flex;

  align-items: center;

  gap: 10px;

}



.table-section {

  flex: 1;

  overflow: auto;

}



.fixed-header {

  position: sticky;

  top: 0;

  background-color: #f5f7fa;

  z-index: 1;

}



.view-name {

  font-weight: bold;

  color: #303133;

}



.view-name.built-in {

  color: #909399;

}



.disabled-button {

  color: #909399;

  border-color: #dcdfe6;

  background-color: #f5f7fa;

}



.starred {

  color: #FFAD15;

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



.column-settings {

  display: flex;

  flex-direction: column;

  gap: 10px;

  padding: 20px 0;

}



:deep(.el-button--primary) {

  background-color: #1677FF;

  border-color: #1677FF;

}



:deep(.el-button--danger) {

  background-color: #FF4D4F;

  border-color: #FF4D4F;

}



:deep(.el-tab-pane) {

  padding-top: 20px;

}

</style>