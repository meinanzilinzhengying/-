<template>

  <div class="metrics-search">

 <!-- 指标搜索工具 -->

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

    

 <!-- 拓扑图展示和拓扑图选择 -->

    <div class="data-source">

      <el-form :inline="true" :model="dataSourceForm" class="data-source-form">

        <el-form-item label="拓扑图展示">

          <el-select v-model="dataSourceForm.database" @change="handleDatabaseChange">

            <el-option label="Prometheus" value="prometheus" />

            <el-option label="DeepFlow" value="deepflow" />

          </el-select>

        </el-form-item>

        <el-form-item label="数据库源">

          <el-select v-model="dataSourceForm.dataTable" @change="handleDataTableChange">

            <el-option label="service_metrics" value="service_metrics" />

            <el-option label="host_metrics" value="host_metrics" />

            <el-option label="container_metrics" value="container_metrics" />

          </el-select>

        </el-form-item>

      </el-form>

    </div>

    

 <!-- 模式切换 -->

    <div class="mode-switch">

      <el-radio-group v-model="searchMode" @change="handleModeChange">

        <el-radio-button label="visual">以图表方式展示</el-radio-button>

        <el-radio-button label="promql">PromQL 模式</el-radio-button>

      </el-radio-group>

    </div>

    

 <!-- 可视化搜索模型-->

    <div v-if="searchMode === 'visual'" class="visual-mode">

 <!-- 指标查询条件-->

      <div class="metrics-groups">

        <div 

          v-for="(group, groupIndex) in metricsGroups" 

          :key="groupIndex"

          class="metrics-group"

        >

          <div class="group-header">

            <span class="group-title">指标查询 {{ groupIndex + 1 }}</span>

            <div class="group-actions">

              <el-switch v-model="group.enabled" @change="handleGroupEnabledChange(groupIndex)">

                聚合方式

              </el-switch>

              <el-button size="small" type="danger" @click="removeMetricsGroup(groupIndex)">

                删除标签

              </el-button>

            </div>

          </div>

          

          <div class="group-content">

 <!-- 指标选择 -->

            <el-form :inline="true" :model="group" class="metrics-form">

              <el-form-item label="指标">

                <el-select v-model="group.metric" placeholder="选择指标" @change="handleMetricChange(groupIndex)">

                  <el-option v-for="metric in metrics" :key="metric.value" :label="metric.label" :value="metric.value" />

                </el-select>

              </el-form-item>

              

 <!-- 运算符选择 -->

              <el-form-item label="运算符">

                <el-select v-model="group.operator" placeholder="选择运算符">

                  <el-option label="Avg" value="avg" />

                  <el-option label="Sum" value="sum" />

                  <el-option label="Max" value="max" />

                  <el-option label="Min" value="min" />

                  <el-option label="P95" value="p95" />

                  <el-option label="P99" value="p99" />

                </el-select>

              </el-form-item>

              

 <!-- 二级运算符选择 -->

              <el-form-item label="二级运算符">

                <el-select v-model="group.secondaryOperator" placeholder="选择二级运算符">

                  <el-option label="Rate" value="rate" />

                  <el-option label="Delta" value="delta" />

                  <el-option label="Increase" value="increase" />

                </el-select>

              </el-form-item>

            </el-form>

            

 <!-- 过滤条件 -->

            <div class="filter-conditions">

              <h4>过滤条件</h4>

              <div 

                v-for="(condition, conditionIndex) in group.conditions" 

                :key="conditionIndex"

                class="condition-item"

              >

                <el-form :inline="true" :model="condition" class="condition-form">

                  <el-form-item label="Tag">

                    <el-select v-model="condition.tag" placeholder="选择Tag">

                      <el-option v-for="tag in tags" :key="tag.value" :label="tag.label" :value="tag.value" />

                    </el-select>

                  </el-form-item>

                  <el-form-item label="条件聚合函数">

                    <el-select v-model="condition.operator" placeholder="选择比较运算符">

                      <el-option label="=" value="eq" />

                      <el-option label="!=" value="ne" />

                      <el-option label=":" value="contains" />

                      <el-option label="!:" value="not_contains" />

                      <el-option label="~" value="regex" />

                      <el-option label="!~" value="not_regex" />

                      <el-option label=">=" value="gte" />

                      <el-option label="<=" value="lte" />

                      <el-option label=">" value="gt" />

                      <el-option label="<" value="lt" />

                    </el-select>

                  </el-form-item>

                  <el-form-item label="值">

                    <el-input v-model="condition.value" placeholder="输入条件值"></el-input>

                  </el-form-item>

                  <el-form-item>

                    <el-button size="small" type="danger" @click="removeCondition(groupIndex, conditionIndex)">

                      删除标签

                    </el-button>

                  </el-form-item>

                </el-form>

              </div>

              <el-button size="small" @click="addCondition(groupIndex)">

                添加过滤条件

              </el-button>

            </div>

          </div>

        </div>

      </div>

      

 <!-- 添加指标操作区 -->

      <div class="add-metrics">

        <el-button type="primary" @click="addMetricsGroup">

          <el-icon><Plus /></el-icon>

          添加指标

        </el-button>

      </div>

    </div>

    

 <!-- PromQL 模式 -->

    <div v-else-if="searchMode === 'promql'" class="promql-mode">

      <div class="promql-input">

        <el-input

          v-model="promqlQuery"

          type="textarea"

          placeholder="输入并支持 PromQL 查询"

          :rows="4"

        ></el-input>

      </div>

      <div class="promql-actions">

        <el-button type="primary" @click="search">

          <el-icon><Search /></el-icon>

          聚合方式

        </el-button>

        <el-button @click="loadExample">

          服务选择框

        </el-button>

      </div>

    </div>

    

 <!-- 搜索标签 -->

    <div v-if="searchTags.length > 0" class="search-tags">

      <h4>搜索标签</h4>

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

</template>



<script setup lang="ts">

import { ref, reactive } from 'vue'

import { ArrowDown, ArrowUp, Search, Plus } from '@element-plus/icons-vue'



// 解析路径数据
const expanded = ref(true)



// 搜索处理逻辑

const searchMode = ref('visual')



// 拓扑图展示和拓扑图选择

const dataSourceForm = reactive({

  database: 'prometheus',

  dataTable: 'service_metrics'

})



// 指标列表

const metrics = ref([

  { label: 'response_time', value: 'response_time' },

  { label: 'request_count', value: 'request_count' },

  { label: 'error_count', value: 'error_count' },

  { label: 'cpu_usage', value: 'cpu_usage' },

  { label: 'memory_usage', value: 'memory_usage' }

])



// Tag列表

const tags = ref([

  { label: 'pod_ns', value: 'pod_ns' },

  { label: 'service', value: 'service' },

  { label: 'region', value: 'region' },

  { label: 'host', value: 'host' },

  { label: 'container', value: 'container' }

])



// 指标查询条件

const metricsGroups = ref([

  {

    enabled: true,

    metric: '',

    operator: 'avg',

    secondaryOperator: '',

    conditions: [

      {

        tag: '',

        operator: 'eq',

        value: ''

      }

    ]

  }

])



// PromQL 后端

const promqlQuery = ref('')



// 搜索标签

const searchTags = ref<any[]>([])



// 性能分析结果展示
const handleDatabaseChange = () => {

 // // 搜索结果与数据详情功能

}



// 处理拓扑图布局渲染
const handleDataTableChange = () => {

 // 实现路径拓扑图相关事件

}



// 处理模式切换

const handleModeChange = () => {

 // 实现模式设置相关事件

}



// 处理指标变化

const handleMetricChange = (groupIndex: number) => {

 // 实现指标变化相关事件待开发

}



// 处理查询组使用状态变化

const handleGroupEnabledChange = (groupIndex: number) => {

 // 实现查询组使用状态变化逻辑

}



// 添加指标查询条件

const addMetricsGroup = () => {

  metricsGroups.value.push({

    enabled: true,

    metric: '',

    operator: 'avg',

    secondaryOperator: '',

    conditions: [

      {

        tag: '',

        operator: 'eq',

        value: ''

      }

    ]

  })

  }



// 删除指标查询条件

const removeMetricsGroup = (groupIndex: number) => {

  metricsGroups.value.splice(groupIndex, 1)

  }



// 添加过滤条件

const addCondition = (groupIndex: number) => {

  metricsGroups.value[groupIndex].conditions.push({

    tag: '',

    operator: 'eq',

    value: ''

  })

  }



// 删除连接事件

const removeCondition = (groupIndex: number, conditionIndex: number) => {

  metricsGroups.value[groupIndex].conditions.splice(conditionIndex, 1)

  }



// 最近5分钟

const search = () => {

 // 实现搜索功能待开发

}



// 时间间隔

const clearSearch = () => {

  metricsGroups.value = [

    {

      enabled: true,

      metric: '',

      operator: 'avg',

      secondaryOperator: '',

      conditions: [

        {

          tag: '',

          operator: 'eq',

          value: ''

        }

      ]

    }

  ]

  promqlQuery.value = ''

  searchTags.value = []

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



// 服务选择框

const loadExample = () => {

  promqlQuery.value = 'sum(rate(http_requests_total{job="api-server"}[5m])) by (status_code)'

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

.metrics-search {

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



.mode-switch {

  margin-bottom: 16px;

}



.visual-mode,

.promql-mode {

  margin-bottom: 16px;

}



.metrics-groups {

  margin-bottom: 16px;

}



.metrics-group {

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  padding: 16px;

  margin-bottom: 16px;

  background-color: #f9f9f9;

}



.group-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 16px;

  padding-bottom: 12px;

  border-bottom: 1px solid #e4e7ed;

}



.group-title {

  font-weight: 500;

  font-size: 14px;

}



.group-actions {

  display: flex;

  gap: 10px;

  align-items: center;

}



.group-content {

  margin-top: 16px;

}



.metrics-form {

  display: flex;

  flex-wrap: wrap;

  gap: 10px;

  align-items: end;

  margin-bottom: 16px;

}



.filter-conditions {

  margin-top: 16px;

}



.filter-conditions h4 {

  margin: 0 0 8px 0;

  font-size: 14px;

  font-weight: 500;

}



.condition-item {

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  padding: 12px;

  margin-bottom: 12px;

  background-color: white;

}



.condition-form {

  display: flex;

  flex-wrap: wrap;

  gap: 10px;

  align-items: end;

}



.add-metrics {

  margin-top: 16px;

  text-align: right;

}



.promql-input {

  margin-bottom: 16px;

}



.promql-actions {

  display: flex;

  gap: 10px;

  justify-content: flex-end;

}



.search-tags {

  margin-top: 16px;

  border-top: 1px solid #e4e7ed;

  padding-top: 16px;

}



.search-tags h4 {

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

  margin-bottom: 8px;

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



:deep(.el-button--primary) {

  background-color: #1677FF;

  border-color: #1677FF;

}



:deep(.el-button--danger) {

  background-color: #FF4D4F;

  border-color: #FF4D4F;

}

</style>