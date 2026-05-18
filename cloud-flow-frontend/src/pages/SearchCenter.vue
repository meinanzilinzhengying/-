<template>
  <div class="search-center">
    <el-page-header title="搜索中心" content="统一数据搜索入口" />
    
    <el-tabs v-model="activeTab" class="search-tabs" type="border-card">
      <el-tab-pane label="综合搜索" name="all">
        <div class="search-section">
          <el-input
            v-model="globalSearchQuery"
            placeholder="输入关键词搜索资源、路径、日志、指标..."
            class="global-search-input"
            size="large"
            clearable
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
            <template #append>
              <el-button type="primary" @click="handleGlobalSearch">搜索</el-button>
            </template>
          </el-input>
          
          <div class="quick-filters">
            <el-tag
              v-for="type in searchTypes"
              :key="type.value"
              :type="selectedTypes.includes(type.value) ? 'primary' : 'info'"
              class="filter-tag"
              @click="toggleSearchType(type.value)"
            >
              {{ type.label }}
            </el-tag>
          </div>
          
          <div class="search-results" v-if="globalSearchQuery">
            <el-empty description="请输入关键词开始搜索" v-if="!hasResults" />
            <div v-else class="results-content">
              <el-alert title="搜索结果将聚合显示各类数据" type="info" :closable="false" />
            </div>
          </div>
        </div>
      </el-tab-pane>
      
      <el-tab-pane label="资源搜索" name="resource">
        <ResourceSearch />
      </el-tab-pane>
      
      <el-tab-pane label="路径搜索" name="path">
        <PathSearch />
      </el-tab-pane>
      
      <el-tab-pane label="日志搜索" name="log">
        <el-empty description="日志搜索功能开发中" />
      </el-tab-pane>
      
      <el-tab-pane label="指标搜索" name="metrics">
        <MetricsSearch />
      </el-tab-pane>
      
      <el-tab-pane label="搜索快照" name="snapshot">
        <SearchSnapshot />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { Search } from '@element-plus/icons-vue'
import ResourceSearch from '../components/ResourceSearch.vue'
import PathSearch from '../components/PathSearch.vue'
// import LogSearch from '../components/LogSearch.vue'
import MetricsSearch from '../components/MetricsSearch.vue'
import SearchSnapshot from '../components/SearchSnapshot.vue'

const activeTab = ref('all')
const globalSearchQuery = ref('')
const selectedTypes = ref<string[]>(['resource', 'path', 'log', 'metrics'])

const searchTypes = [
  { label: '资源', value: 'resource' },
  { label: '路径', value: 'path' },
  { label: '日志', value: 'log' },
  { label: '指标', value: 'metrics' }
]

const hasResults = computed(() => {
  return globalSearchQuery.value.length > 0
})

const toggleSearchType = (type: string) => {
  const index = selectedTypes.value.indexOf(type)
  if (index > -1) {
    selectedTypes.value.splice(index, 1)
  } else {
    selectedTypes.value.push(type)
  }
}

const handleGlobalSearch = () => {
  // 触发综合搜索
  console.log('Global search:', globalSearchQuery.value, 'types:', selectedTypes.value)
}
</script>

<style scoped>
.search-center {
  padding: 20px;
}

.search-tabs {
  margin-top: 20px;
}

.search-section {
  padding: 20px;
}

.global-search-input {
  max-width: 800px;
  margin: 0 auto 20px;
  display: block;
}

.quick-filters {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-bottom: 30px;
}

.filter-tag {
  cursor: pointer;
  transition: all 0.3s;
}

.filter-tag:hover {
  transform: scale(1.05);
}

.search-results {
  min-height: 300px;
}

.results-content {
  padding: 20px;
}
</style>
