<template>
  <div class="analysis-center">
    <el-page-header title="分析中心" content="网络与应用统一分析入口" />
    
    <el-tabs v-model="activeTab" class="analysis-tabs" type="border-card">
      <el-tab-pane label="资源分析" name="resource">
        <div class="analysis-content">
          <el-radio-group v-model="resourceScope" size="small" class="scope-selector">
            <el-radio-button label="infrastructure">基础设施</el-radio-button>
            <el-radio-button label="application">应用资源</el-radio-button>
          </el-radio-group>
          <ResourceAnalysis :scope="resourceScope" />
        </div>
      </el-tab-pane>
      
      <el-tab-pane label="路径分析" name="path">
        <div class="analysis-content">
          <el-radio-group v-model="pathScope" size="small" class="scope-selector">
            <el-radio-button label="network">网络路径</el-radio-button>
            <el-radio-button label="application">应用路径</el-radio-button>
          </el-radio-group>
          <PathAnalysis :scope="pathScope" />
        </div>
      </el-tab-pane>
      
      <el-tab-pane label="拓扑分析" name="topology">
        <div class="analysis-content">
          <el-radio-group v-model="topologyScope" size="small" class="scope-selector">
            <el-radio-button label="network">网络拓扑</el-radio-button>
            <el-radio-button label="application">应用拓扑</el-radio-button>
            <el-radio-button label="service">服务拓扑</el-radio-button>
          </el-radio-group>
          <TopologyAnalysis :scope="topologyScope" />
        </div>
      </el-tab-pane>
      
      <el-tab-pane label="流日志" name="flow">
        <FlowAnalysis />
      </el-tab-pane>
      
      <el-tab-pane label="NAT追踪" name="nat">
        <NatTracing />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ResourceAnalysis from '../components/network/ResourceAnalysis.vue'
import PathAnalysis from '../components/network/PathAnalysis.vue'
import TopologyAnalysis from '../components/network/TopologyAnalysis.vue'
import FlowAnalysis from '../components/network/FlowAnalysis.vue'
import NatTracing from '../components/network/NATAnalysis.vue'

const activeTab = ref('resource')
const resourceScope = ref('infrastructure')
const pathScope = ref('network')
const topologyScope = ref('network')
</script>

<style scoped>
.analysis-center {
  padding: 20px;
}

.analysis-tabs {
  margin-top: 20px;
}

.analysis-content {
  padding: 20px;
}

.scope-selector {
  margin-bottom: 20px;
  display: flex;
  justify-content: center;
}
</style>
