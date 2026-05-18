<template>
  <div class="log-center">
    <el-page-header title="日志中心" content="统一日志查询与分析入口" />
    
    <el-tabs v-model="activeTab" class="log-tabs" type="border-card">
      <el-tab-pane label="日志概览" name="overview">
        <div class="overview-content">
          <el-row :gutter="20">
            <el-col :span="6">
              <el-statistic title="今日日志总量" :value="268500" />
            </el-col>
            <el-col :span="6">
              <el-statistic title="错误日志数" :value="1250" value-style="color: #f56c6c" />
            </el-col>
            <el-col :span="6">
              <el-statistic title="警告日志数" :value="3890" value-style="color: #e6a23c" />
            </el-col>
            <el-col :span="6">
              <el-statistic title="采集器状态" value="正常" value-style="color: #67c23a" />
            </el-col>
          </el-row>
          
          <el-divider />
          
          <div class="quick-links">
            <h4>快速入口</h4>
            <el-space>
              <el-button type="primary" @click="activeTab = 'system'">系统日志</el-button>
              <el-button type="success" @click="activeTab = 'application'">应用日志</el-button>
              <el-button type="warning" @click="activeTab = 'search'">日志搜索</el-button>
              <el-button type="info" @click="activeTab = 'flow'">流日志</el-button>
            </el-space>
          </div>
        </div>
      </el-tab-pane>
      
      <el-tab-pane label="系统日志" name="system">
        <SystemLog />
      </el-tab-pane>
      
      <el-tab-pane label="应用日志" name="application">
        <ApplicationLog />
      </el-tab-pane>
      
      <el-tab-pane label="日志搜索" name="search">
        <el-empty description="日志搜索功能开发中" />
      </el-tab-pane>
      
      <el-tab-pane label="流日志" name="flow">
        <FlowLog />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import SystemLog from './system/SystemLog.vue'
import ApplicationLog from './AppLog.vue'
// import LogSearch from '../components/LogSearch.vue'
import FlowLog from '../components/network/FlowAnalysis.vue'

const activeTab = ref('overview')
</script>

<style scoped>
.log-center {
  padding: 20px;
}

.log-tabs {
  margin-top: 20px;
}

.overview-content {
  padding: 20px;
}

.quick-links {
  margin-top: 30px;
}

.quick-links h4 {
  margin-bottom: 15px;
}
</style>
