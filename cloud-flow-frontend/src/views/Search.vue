<template>
  <div class="search">
    <div class="page-header">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item><router-link to="/">管理后台</router-link></el-breadcrumb-item>
        <el-breadcrumb-item>数据库搜索</el-breadcrumb-item>
      </el-breadcrumb>
      <h2>数据库搜索</h2>
    </div>
    
    <div class="search-content">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>数据库搜索模块</span>
          </div>
        </template>
        <div class="search-modules">
          <el-row :gutter="20">
            <el-col :span="6">
              <el-card class="module-card">
                <template #header>
                  <div class="module-header">
                    <el-icon><Search /></el-icon>
                    <span>资源搜索框</span>
                  </div>
                </template>
                <div class="module-content">
                  <p>支持自由/容器/进程三种输入模式，可切换</p>
                  <el-button type="primary" @click="navigateTo('resource')">进入</el-button>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card class="module-card">
                <template #header>
                  <div class="module-header">
                    <el-icon><Link /></el-icon>
                    <span>路径搜索框</span>
                  </div>
                </template>
                <div class="module-content">
                  <p>支持精简/单向/双向三种路径模式</p>
                  <el-button type="primary" @click="navigateTo('path')">进入</el-button>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card class="module-card">
                <template #header>
                  <div class="module-header">
                    <el-icon><Message /></el-icon>
                    <span>日志搜索框</span>
                  </div>
                </template>
                <div class="module-content">
                  <p>支持按时间、级别、关键词等多维度搜索</p>
                  <el-button type="primary" @click="navigateTo('log')">进入</el-button>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card class="module-card">
                <template #header>
                  <div class="module-header">
                    <el-icon><DataAnalysis /></el-icon>
                    <span>指标搜索框</span>
                  </div>
                </template>
                <div class="module-content">
                  <p>支持可视化搜索与PromQL脚本模式</p>
                  <el-button type="primary" @click="navigateTo('metrics')">进入</el-button>
                </div>
              </el-card>
            </el-col>
          </el-row>
          
          <el-row :gutter="20" style="margin-top: 20px;">
            <el-col :span="6">
              <el-card class="module-card">
                <template #header>
                  <div class="module-header">
                    <el-icon><Collection /></el-icon>
                    <span>搜索快照</span>
                  </div>
                </template>
                <div class="module-content">
                  <p>保存和管理搜索条件，快速应用</p>
                  <el-button type="primary" @click="navigateTo('snapshot')">进入</el-button>
                </div>
              </el-card>
            </el-col>

          </el-row>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Search, Link, Message, DataAnalysis, Collection } from '@element-plus/icons-vue'

const router = useRouter()

const navigateTo = (path: string) => {
  router.push(`/search/${path}`)
}
</script>

<style scoped>
.search {
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
  padding-bottom: 16px;
  border-bottom: 1px solid #e4e7ed;
}

.page-header h2 {
  margin: 8px 0 0 0;
  font-size: 18px;
  font-weight: bold;
  color: #303133;
}

.search-content {
  flex: 1;
  overflow: auto;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.search-modules {
  margin-top: 20px;
}

.module-card {
  height: 100%;
  transition: all 0.3s;
}

.module-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.module-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.module-content {
  padding: 20px 0;
}

.module-content p {
  margin: 0 0 16px 0;
  color: #606266;
  line-height: 1.4;
}

:deep(.el-button--primary) {
  background-color: #1677FF;
  border-color: #1677FF;
}
</style>