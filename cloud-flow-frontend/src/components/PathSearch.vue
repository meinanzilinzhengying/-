<template>

  <div class="path-search">

 <!-- 路径搜索工具 -->

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

    

 <!-- 数据库源信号源选择 -->

    <div class="data-source">

      <el-form :inline="true" :model="dataSourceForm" class="data-source-form">

        <el-form-item label="数据库源">

          <el-select v-model="dataSourceForm.dataTable" @change="handleDataTableChange">

            <el-option label="分钟粒度" value="minute" />

            <el-option label="秒级粒度" value="second" />

            <el-option label="调用日志" value="call_log" />

          </el-select>

        </el-form-item>

        <el-form-item label="信号源">

          <el-select v-model="dataSourceForm.signalSource" multiple>

            <el-option label="eBPF" value="ebpf" />

            <el-option label="Packet" value="packet" />

            <el-option label="OTel" value="otel" />

          </el-select>

        </el-form-item>

      </el-form>

    </div>

    

 <!-- 搜索模式切换 -->

    <div class="search-mode">

      <el-radio-group v-model="searchMode" @change="handleModeChange">

        <el-radio-button label="simple">简洁模式</el-radio-button>

        <el-radio-button label="one-way">单向路径</el-radio-button>

        <el-radio-button label="two-way">双向路径</el-radio-button>

      </el-radio-group>

    </div>

    

 <!-- 绮剧畝模式 -->

    <div v-if="searchMode === 'simple'" class="simple-mode">

      <div class="search-input">

        <el-input

          v-model="simpleSearchInput"

          placeholder="输入搜索条件，例如：pod_ns: default"

          @input="handleSimpleInput"

          @keydown.enter="search"

        >

          <template #append>

            <el-button @click="search">

              <el-icon><Search /></el-icon>

            </el-button>

          </template>

        </el-input>

      </div>

      

 <!-- 路径过滤器 -->

      <div class="path-filter">

        <el-form-item label="路径过滤器">

          <el-select v-model="pathFilter" multiple>

            <el-option label="服务内部" value="service_internal" />

            <el-option label="服务外部" value="service_external" />

            <el-option label="系统管理" value="wan" />

          </el-select>

        </el-form-item>

      </div>

      

 <!-- 联想建议 -->

      <div v-if="simpleSuggestions.length > 0" class="suggestions">

        <div 

          v-for="suggestion in simpleSuggestions" 

          :key="suggestion.value"

          class="suggestion-item"

          @click="selectSimpleSuggestion(suggestion)"

        >

          <span class="suggestion-text">{{ suggestion.label }}</span>

          <span class="suggestion-desc">{{ suggestion.desc }}</span>

        </div>

      </div>

      

 <!-- 搜索标签 -->

      <div v-if="simpleSearchTags.length > 0" class="search-tags">

        <div 

          v-for="(tag, index) in simpleSearchTags" 

          :key="index"

          class="tag-item"

        >

          <span class="tag-text">{{ tag.key }} {{ tag.operator }} {{ tag.value }}</span>

          <div class="tag-actions">

            <el-button size="small" @click="disableSimpleTag(index)">

              禁用标签

            </el-button>

            <el-button size="small" @click="editSimpleTag(index)">

              编辑

            </el-button>

            <el-button size="small" type="danger" @click="removeSimpleTag(index)">

              删除标签

            </el-button>

          </div>

        </div>

      </div>

    </div>

    

 <!-- 单向路径模式 -->

    <div v-else-if="searchMode === 'one-way'" class="one-way-mode">

      <div class="direction-selector">

        <el-radio-group v-model="direction">

          <el-radio-button label="client">客户端服务</el-radio-button>

          <el-radio-button label="server">服务端列表</el-radio-button>

        </el-radio-group>

        <el-button @click="swapDirection">

          <el-icon><Refresh /></el-icon>

          路径分析列表

        </el-button>

      </div>

      

      <div class="search-input">

        <el-input

          v-model="oneWaySearchInput"

          placeholder="输入搜索条件，例如：pod_ns: default"

          @input="handleOneWayInput"

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

      <div v-if="oneWaySuggestions.length > 0" class="suggestions">

        <div 

          v-for="suggestion in oneWaySuggestions" 

          :key="suggestion.value"

          class="suggestion-item"

          @click="selectOneWaySuggestion(suggestion)"

        >

          <span class="suggestion-text">{{ suggestion.label }}</span>

          <span class="suggestion-desc">{{ suggestion.desc }}</span>

        </div>

      </div>

      

 <!-- 搜索标签 -->

      <div v-if="oneWaySearchTags.length > 0" class="search-tags">

        <div 

          v-for="(tag, index) in oneWaySearchTags" 

          :key="index"

          class="tag-item"

        >

          <span class="tag-text">{{ tag.key }} {{ tag.operator }} {{ tag.value }}</span>

          <div class="tag-actions">

            <el-button size="small" @click="disableOneWayTag(index)">

              禁用标签

            </el-button>

            <el-button size="small" @click="editOneWayTag(index)">

              编辑

            </el-button>

            <el-button size="small" type="danger" @click="removeOneWayTag(index)">

              删除标签

            </el-button>

          </div>

        </div>

      </div>

    </div>

    

 <!-- 双向路径模式 -->

    <div v-else-if="searchMode === 'two-way'" class="two-way-mode">

      <div class="search-inputs">

        <div class="client-search">

          <el-input

            v-model="twoWayClientInput"

            placeholder="客户端搜索条件"

            @input="handleTwoWayClientInput"

            @keydown.enter="search"

          ></el-input>

        </div>

        <div class="server-search">

          <el-input

            v-model="twoWayServerInput"

            placeholder="服务端搜索条件"

            @input="handleTwoWayServerInput"

            @keydown.enter="search"

          ></el-input>

        </div>

        <el-button type="primary" @click="search">

          <el-icon><Search /></el-icon>搜索</el-button>

      </div>

      

 <!-- 联想建议 -->

      <div v-if="twoWayClientSuggestions.length > 0" class="suggestions">

        <div 

          v-for="suggestion in twoWayClientSuggestions" 

          :key="suggestion.value"

          class="suggestion-item"

          @click="selectTwoWayClientSuggestion(suggestion)"

        >

          <span class="suggestion-text">{{ suggestion.label }}</span>

          <span class="suggestion-desc">{{ suggestion.desc }}</span>

        </div>

      </div>

      

      <div v-if="twoWayServerSuggestions.length > 0" class="suggestions">

        <div 

          v-for="suggestion in twoWayServerSuggestions" 

          :key="suggestion.value"

          class="suggestion-item"

          @click="selectTwoWayServerSuggestion(suggestion)"

        >

          <span class="suggestion-text">{{ suggestion.label }}</span>

          <span class="suggestion-desc">{{ suggestion.desc }}</span>

        </div>

      </div>

      

 <!-- 搜索标签 -->

      <div v-if="twoWayClientTags.length > 0" class="search-tags">

        <h4>路径节点连接详情</h4>

        <div 

          v-for="(tag, index) in twoWayClientTags" 

          :key="index"

          class="tag-item"

        >

          <span class="tag-text">{{ tag.key }} {{ tag.operator }} {{ tag.value }}</span>

          <div class="tag-actions">

            <el-button size="small" @click="disableTwoWayClientTag(index)">

              禁用标签

            </el-button>

            <el-button size="small" @click="editTwoWayClientTag(index)">

              编辑

            </el-button>

            <el-button size="small" type="danger" @click="removeTwoWayClientTag(index)">

              删除标签

            </el-button>

          </div>

        </div>

      </div>

      

      <div v-if="twoWayServerTags.length > 0" class="search-tags">

        <h4>默认拓扑详情</h4>

        <div 

          v-for="(tag, index) in twoWayServerTags" 

          :key="index"

          class="tag-item"

        >

          <span class="tag-text">{{ tag.key }} {{ tag.operator }} {{ tag.value }}</span>

          <div class="tag-actions">

            <el-button size="small" @click="disableTwoWayServerTag(index)">

              禁用标签

            </el-button>

            <el-button size="small" @click="editTwoWayServerTag(index)">

              编辑

            </el-button>

            <el-button size="small" type="danger" @click="removeTwoWayServerTag(index)">

              删除标签

            </el-button>

          </div>

        </div>

      </div>

    </div>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive } from 'vue'

import { ArrowDown, ArrowUp, Search, Refresh } from '@element-plus/icons-vue'



// 解析路径数据
const expanded = ref(true)



// 搜索处理逻辑

const searchMode = ref('simple')



// 新路径

const direction = ref('client')



// 数据库源信号源表单

const dataSourceForm = reactive({

  dataTable: 'minute',

  signalSource: []

})



// 路径过滤器

const pathFilter = ref([])



// 绮剧畝模式

const simpleSearchInput = ref('')

const simpleSuggestions = ref<any[]>([])

const simpleSearchTags = ref<any[]>([])



// 单向路径模式

const oneWaySearchInput = ref('')

const oneWaySuggestions = ref<any[]>([])

const oneWaySearchTags = ref<any[]>([])



// 双向路径模式

const twoWayClientInput = ref('')

const twoWayServerInput = ref('')

const twoWayClientSuggestions = ref<any[]>([])

const twoWayServerSuggestions = ref<any[]>([])

const twoWayClientTags = ref<any[]>([])

const twoWayServerTags = ref<any[]>([])



// 处理拓扑图布局渲染
const handleDataTableChange = () => {

 // 实现路径拓扑图相关事件

}



// 处理模式切换

const handleModeChange = () => {

 // 实现模式设置相关事件

}



// 路径分析列表

const swapDirection = () => {

  direction.value = direction.value === 'client' ? 'server' : 'client'

  }



// 处理简洁模式数据

const handleSimpleInput = () => {

 // 模拟联想联想

  if (simpleSearchInput.value.length > 0) {

    simpleSuggestions.value = [

      { label: 'pod_ns: default', desc: '命名空间' },

      { label: 'service: web-shop', desc: '服务' },

      { label: 'region: 区域1', desc: '区域' }

    ]

  } else {

    simpleSuggestions.value = []

  }

}



// 选择简洁模式建议

const selectSimpleSuggestion = (suggestion: any) => {

  simpleSearchInput.value = suggestion.label

  simpleSuggestions.value = []

  

 // 解析标签并添加节点和边

  const parts = suggestion.label.split(': ')

  if (parts.length === 2) {

    simpleSearchTags.value.push({

      key: parts[0],

      operator: ':',

      value: parts[1]

    })

    simpleSearchInput.value = ''

  }

}



// 处理单向路径输入

const handleOneWayInput = () => {

 // 模拟联想联想

  if (oneWaySearchInput.value.length > 0) {

    oneWaySuggestions.value = [

      { label: 'pod_ns: default', desc: '命名空间' },

      { label: 'service: web-shop', desc: '服务' },

      { label: 'region: 区域1', desc: '区域' }

    ]

  } else {

    oneWaySuggestions.value = []

  }

}



// 选择单向路径模式

const selectOneWaySuggestion = (suggestion: any) => {

  oneWaySearchInput.value = suggestion.label

  oneWaySuggestions.value = []

  

 // 解析标签并添加节点和边

  const parts = suggestion.label.split(': ')

  if (parts.length === 2) {

    oneWaySearchTags.value.push({

      key: parts[0],

      operator: ':',

      value: parts[1]

    })

    oneWaySearchInput.value = ''

  }

}



// 处理双向路径拓扑数据
const handleTwoWayClientInput = () => {

 // 模拟联想联想

  if (twoWayClientInput.value.length > 0) {

    twoWayClientSuggestions.value = [

      { label: 'pod_ns: default', desc: '命名空间' },

      { label: 'service: web-shop', desc: '服务' },

      { label: 'region: 区域1', desc: '区域' }

    ]

  } else {

    twoWayClientSuggestions.value = []

  }

}



// 选择选择客户端服务

const selectTwoWayClientSuggestion = (suggestion: any) => {

  twoWayClientInput.value = suggestion.label

  twoWayClientSuggestions.value = []

  

 // 解析标签并添加节点和边

  const parts = suggestion.label.split(': ')

  if (parts.length === 2) {

    twoWayClientTags.value.push({

      key: parts[0],

      operator: ':',

      value: parts[1]

    })

    twoWayClientInput.value = ''

  }

}



// 处理双向路径服务调用关系

const handleTwoWayServerInput = () => {

 // 模拟联想联想

  if (twoWayServerInput.value.length > 0) {

    twoWayServerSuggestions.value = [

      { label: 'pod_ns: default', desc: '命名空间' },

      { label: 'service: web-shop', desc: '服务' },

      { label: 'region: 区域1', desc: '区域' }

    ]

  } else {

    twoWayServerSuggestions.value = []

  }

}



// 选择选择服务端名称

const selectTwoWayServerSuggestion = (suggestion: any) => {

  twoWayServerInput.value = suggestion.label

  twoWayServerSuggestions.value = []

  

 // 解析标签并添加节点和边

  const parts = suggestion.label.split(': ')

  if (parts.length === 2) {

    twoWayServerTags.value.push({

      key: parts[0],

      operator: ':',

      value: parts[1]

    })

    twoWayServerInput.value = ''

  }

}



// 最近5分钟

const search = () => {

 // 实现搜索功能待开发

}



// 时间间隔

const clearSearch = () => {

  simpleSearchInput.value = ''

  simpleSuggestions.value = []

  simpleSearchTags.value = []

  oneWaySearchInput.value = ''

  oneWaySuggestions.value = []

  oneWaySearchTags.value = []

  twoWayClientInput.value = ''

  twoWayServerInput.value = ''

  twoWayClientSuggestions.value = []

  twoWayServerSuggestions.value = []

  twoWayClientTags.value = []

  twoWayServerTags.value = []

  pathFilter.value = []

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



// 禁用标签

const disableSimpleTag = (index: number) => {

 // 解析并实现相关事件

}



// 编辑标签

const editSimpleTag = (index: number) => {

 // 实现编辑拓扑图功能

}



// 创建标签

const removeSimpleTag = (index: number) => {

  simpleSearchTags.value.splice(index, 1)

  }



// 禁用单向路径标签

const disableOneWayTag = (index: number) => {

 // 解析并实现相关事件

}



// 编辑单向路径标签

const editOneWayTag = (index: number) => {

 // 实现编辑拓扑图功能

}



// 创建单向路径标签

const removeOneWayTag = (index: number) => {

  oneWaySearchTags.value.splice(index, 1)

  }



// 禁用双向路径基础详细视图

const disableTwoWayClientTag = (index: number) => {

 // 解析并实现相关事件

}



// 编辑双向路径基础详细视图

const editTwoWayClientTag = (index: number) => {

 // 实现编辑拓扑图功能

}



// 删除路径拓扑图详情
const removeTwoWayClientTag = (index: number) => {

  twoWayClientTags.value.splice(index, 1)

  }



// 禁用双向路径高级详细视图

const disableTwoWayServerTag = (index: number) => {

 // 解析并实现相关事件

}



// 编辑双向路径高级详细视图

const editTwoWayServerTag = (index: number) => {

 // 实现编辑拓扑图功能

}



// 删除路径拓扑图云服务节点
const removeTwoWayServerTag = (index: number) => {

  twoWayServerTags.value.splice(index, 1)

  }

</script>



<style scoped>

.path-search {

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



.data-source {

  margin-bottom: 16px;

}



.data-source-form {

  display: flex;

  flex-wrap: wrap;

  gap: 10px;

  align-items: end;

}



.search-mode {

  margin-bottom: 16px;

}



.simple-mode,

.one-way-mode,

.two-way-mode {

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



.search-tags h4 {

  width: 100%;

  margin: 0 0 8px 0;

  font-size: 14px;

  font-weight: 500;

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



.direction-selector {

  display: flex;

  align-items: center;

  gap: 10px;

  margin-bottom: 16px;

}



.search-inputs {

  display: flex;

  gap: 10px;

  flex-wrap: wrap;

  align-items: end;

  margin-bottom: 16px;

}



.client-search,

.server-search {

  flex: 1;

  min-width: 200px;

}



.path-filter {

  margin-bottom: 16px;

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