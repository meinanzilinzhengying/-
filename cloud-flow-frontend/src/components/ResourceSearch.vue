<template>

  <div class="resource-search">

 <!-- 资源搜索工具 -->

    <div class="search-header">

      <div class="header-left">

        <el-dropdown>

          <el-button>

            时间范围 <el-icon class="el-icon--right"><ArrowDown /></el-icon>

          </el-button>

          <template #dropdown>

            <el-dropdown-menu>

              <el-dropdown-item @click="saveSnapshot">保存容器</el-dropdown-item>

              <el-dropdown-item @click="loadSnapshot">加载快照</el-dropdown-item>

            </el-dropdown-menu>

          </template>

        </el-dropdown>

        <el-dropdown>

          <el-button>

            分组依据 <el-icon class="el-icon--right"><ArrowDown /></el-icon>

          </el-button>

          <template #dropdown>

            <el-dropdown-menu>

              <el-dropdown-item @click="setGroup('primary')">主要企业</el-dropdown-item>

              <el-dropdown-item @click="setGroup('secondary')">次要企业</el-dropdown-item>

            </el-dropdown-menu>

          </template>

        </el-dropdown>

        <el-button @click="clearSearch">保存搜索</el-button>

      </div>

      <div class="header-right">

        <el-button @click="toggleExpand">

          <el-icon v-if="expanded"><ArrowUp /></el-icon>

          <el-icon v-else><ArrowDown /></el-icon>

          {{ expanded ? '收起面板' : '展开面板详情' }}

        </el-button>

      </div>

    </div>

    

 <!-- 输入模式切换 -->

    <div class="input-mode">

      <el-radio-group v-model="inputMode" @change="handleModeChange">

        <el-radio-button label="free">空闲</el-radio-button>

        <el-radio-button label="container">容器</el-radio-button>

        <el-radio-button label="process">进程</el-radio-button>

      </el-radio-group>

    </div>

    

 <!-- 自定义模式 -->

    <div v-if="inputMode === 'free'" class="free-search">

      <div class="search-input">

        <el-input

          v-model="searchInput"

          placeholder="输入搜索条件，例如：pod_ns: default"

          @input="handleInput"

          @keydown.enter="search"

        >

          <template #append>

            <el-button @click="search">

              <el-icon><Search /></el-icon>

            </el-button>

          </template>

        </el-input>

      </div>

      

 <!-- 联想建议 -->

      <div v-if="suggestions.length > 0" class="suggestions">

        <div 

          v-for="suggestion in suggestions" 

          :key="suggestion.value"

          class="suggestion-item"

          @click="selectSuggestion(suggestion)"

        >

          <span class="suggestion-text">{{ suggestion.label }}</span>

          <span class="suggestion-desc">{{ suggestion.desc }}</span>

        </div>

      </div>

      

 <!-- 搜索标签 -->

      <div v-if="searchTags.length > 0" class="search-tags">

        <div 

          v-for="(tag, index) in searchTags" 

          :key="index"

          class="tag-item"

        >

          <span class="tag-text">{{ tag.key }} {{ tag.operator }} {{ tag.value }}</span>

          <div class="tag-actions">

            <el-button size="small" @click="disableTag(index)">

              禁用标签

            </el-button>

            <el-button size="small" @click="editTag(index)">

              编辑

            </el-button>

            <el-button size="small" type="danger" @click="removeTag(index)">

              删除标签

            </el-button>

          </div>

        </div>

      </div>

    </div>

    

 <!-- 容器搜索模式 -->

    <div v-else-if="inputMode === 'container'" class="container-search">

      <el-form :inline="true" :model="containerForm" class="container-form">

        <el-form-item label="集群名称">

          <el-select v-model="containerForm.cluster" placeholder="选择集群" @change="handleClusterChange">

            <el-option v-for="cluster in clusters" :key="cluster.value" :label="cluster.label" :value="cluster.value" />

          </el-select>

        </el-form-item>

        <el-form-item label="命名空间">

          <el-select v-model="containerForm.namespace" placeholder="选择命名空间" @change="handleNamespaceChange">

            <el-option v-for="namespace in namespaces" :key="namespace.value" :label="namespace.label" :value="namespace.value" />

          </el-select>

        </el-form-item>

        <el-form-item label="容器工作负载">

          <el-select v-model="containerForm.workload" placeholder="选择工作负载" @change="handleWorkloadChange">

            <el-option v-for="workload in workloads" :key="workload.value" :label="workload.label" :value="workload.value" />

          </el-select>

        </el-form-item>

        <el-form-item label="POD">

          <el-select v-model="containerForm.pod" placeholder="选择POD">

            <el-option v-for="pod in pods" :key="pod.value" :label="pod.label" :value="pod.value" />

          </el-select>

        </el-form-item>

        <el-form-item>

          <el-button type="primary" @click="search">搜索</el-button>

        </el-form-item>

      </el-form>

    </div>

    

 <!-- 进程搜索模式 -->

    <div v-else-if="inputMode === 'process'" class="process-search">

      <el-form :inline="true" :model="processForm" class="process-form">

        <el-form-item label="网络连接状态">

          <el-select v-model="processForm.processName" placeholder="选择进程名">

            <el-option v-for="process in processes" :key="process.value" :label="process.label" :value="process.value" />

          </el-select>

        </el-form-item>

        <el-form-item label="PID">

          <el-input v-model="processForm.pid" placeholder="输入进程名或PID"></el-input>

        </el-form-item>

        <el-form-item label="自定义搜索">

          <el-input v-model="processForm.freeSearch" placeholder="输入搜索条件"></el-input>

        </el-form-item>

        <el-form-item>

          <el-button type="primary" @click="search">搜索</el-button>

        </el-form-item>

      </el-form>

    </div>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive, computed } from 'vue'

import { ArrowDown, ArrowUp, Search } from '@element-plus/icons-vue'



// 拓扑图节点数据

const inputMode = ref('free')



// 解析路径数据
const expanded = ref(true)



// 时间间隔

const searchInput = ref('')



// 搜索标签

const searchTags = ref<any[]>([])



// 联想建议

const suggestions = ref<any[]>([])



// 搜索并过滤结果

const containerForm = reactive({

  cluster: '',

  namespace: '',

  workload: '',

  pod: ''

})



// 网络端口搜索

const processForm = reactive({

  processName: '',

  pid: '',

  freeSearch: ''

})



// 集群列表

const clusters = ref([

  { label: '集群名称1', value: 'cluster1' },

  { label: '集群名称2', value: 'cluster2' },

  { label: '集群名称3', value: 'cluster3' }

])



// 命名空间列表

const namespaces = ref([

  { label: 'default', value: 'default' },

  { label: 'kube-system', value: 'kube-system' },

  { label: 'app', value: 'app' }

])



// 工作负载列表

const workloads = ref([

  { label: 'deployment-1', value: 'deployment-1' },

  { label: 'deployment-2', value: 'deployment-2' },

  { label: 'statefulset-1', value: 'statefulset-1' }

])



// POD列表

const pods = ref([

  { label: 'pod-1', value: 'pod-1' },

  { label: 'pod-2', value: 'pod-2' },

  { label: 'pod-3', value: 'pod-3' }

])



// 进程列表

const processes = ref([

  { label: 'nginx', value: 'nginx' },

  { label: 'mysql', value: 'mysql' },

  { label: 'redis', value: 'redis' }

])



// 性能分析

const handleInput = () => {

 // 模拟联想联想

  if (searchInput.value.length > 0) {

    suggestions.value = [

      { label: 'pod_ns: default', desc: '命名空间' },

      { label: 'service: web-shop', desc: '服务' },

      { label: 'region: 区域1', desc: '区域' }

    ]

  } else {

    suggestions.value = []

  }

}



// 选择建议

const selectSuggestion = (suggestion: any) => {

  searchInput.value = suggestion.label

  suggestions.value = []

  

 // 解析标签并添加节点和边

  const parts = suggestion.label.split(': ')

  if (parts.length === 2) {

    searchTags.value.push({

      key: parts[0],

      operator: ':',

      value: parts[1]

    })

    searchInput.value = ''

  }

}



// 最近5分钟

const search = () => {

 // 实现搜索功能待开发

}



// 时间间隔

const clearSearch = () => {

  searchInput.value = ''

  searchTags.value = []

  containerForm.cluster = ''

  containerForm.namespace = ''

  containerForm.workload = ''

  containerForm.pod = ''

  processForm.processName = ''

  processForm.pid = ''

  processForm.freeSearch = ''

  suggestions.value = []

  }



// 初始化图表和画布

const toggleExpand = () => {

  expanded.value = !expanded.value

}



// 保存容器

const saveSnapshot = () => {

 // 解析标签并添加节点

}



// 加载快照

const loadSnapshot = () => {

 // 解析标签并添加边

}



// 禁止单向路径

const setGroup = (type: string) => {

 // 实现设置下拉相关事件

}



// 处理模式切换

const handleModeChange = () => {

 // 实现模式设置相关事件

}



// 处理集群变化

const handleClusterChange = () => {

 // 根据条件筛选并查询

}



// 处理命名空间变化

const handleNamespaceChange = () => {

 // 实现命名空间切换功能

}



// 处理工作负载变化

const handleWorkloadChange = () => {

 // 实现工作负载切换功能

}



// 禁用标签

const disableTag = (index: number) => {

 // 解析并实现相关事件

}



// 编辑标签

const editTag = (index: number) => {

 // 实现编辑拓扑图功能

}



// 创建标签

const removeTag = (index: number) => {

  searchTags.value.splice(index, 1)

  }

</script>



<style scoped>

.resource-search {

  background-color: white;

  border-radius: 4px;

  padding: 16px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  margin-bottom: 16px;

}



.search-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 16px;

  padding-bottom: 12px;

  border-bottom: 1px solid #e4e7ed;

}



.header-left {

  display: flex;

  gap: 10px;

}



.header-right {

  display: flex;

  gap: 10px;

}



.input-mode {

  margin-bottom: 16px;

}



.free-search {

  margin-bottom: 16px;

}



.search-input {

  margin-bottom: 16px;

}



.suggestions {

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  background-color: white;

  position: absolute;

  z-index: 1000;

  width: 100%;

  max-height: 200px;

  overflow-y: auto;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

}



.suggestion-item {

  padding: 10px 16px;

  cursor: pointer;

  display: flex;

  justify-content: space-between;

}



.suggestion-item:hover {

  background-color: #f5f7fa;

}



.suggestion-text {

  font-weight: 500;

}



.suggestion-desc {

  color: #909399;

  font-size: 12px;

}



.search-tags {

  display: flex;

  flex-wrap: wrap;

  gap: 10px;

  margin-top: 16px;

}



.tag-item {

  display: flex;

  align-items: center;

  gap: 10px;

  background-color: #ecf5ff;

  border: 1px solid #d9ecff;

  border-radius: 4px;

  padding: 6px 12px;

  transition: all 0.3s;

}



.tag-item:hover {

  background-color: #d9ecff;

  box-shadow: 0 2px 8px rgba(22, 119, 255, 0.2);

}



.tag-text {

  color: #1677FF;

  font-weight: 500;

}



.tag-actions {

  display: flex;

  gap: 5px;

  opacity: 0;

  transition: opacity 0.3s;

}



.tag-item:hover .tag-actions {

  opacity: 1;

}



.container-search,

.process-search {

  margin-bottom: 16px;

}



.container-form,

.process-form {

  display: flex;

  flex-wrap: wrap;

  gap: 10px;

  align-items: end;

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