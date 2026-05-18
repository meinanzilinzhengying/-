<template>
  <div class="layout-container">
 <!-- 侧边导航栏 -->
    <el-aside class="layout-aside" width="200px">
      <div class="aside-header">
        <h1 class="logo">云内流量监测</h1>
      </div>
      <el-menu
        :default-active="activeMenu"
        class="aside-menu"
        background-color="#001529"
        text-color="#fff"
        active-text-color="#409EFF"
        router
      >
        <!-- 聚合搜索中心 -->
        <el-menu-item index="/search-center">
          <el-icon><Search /></el-icon>
          <template #title>搜索中心</template>
        </el-menu-item>
        <!-- 聚合分析中心 -->
        <el-menu-item index="/analysis-center">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>分析中心</template>
        </el-menu-item>
        <!-- 聚合日志中心 -->
        <el-menu-item index="/log-center">
          <el-icon><Document /></el-icon>
          <template #title>日志中心</template>
        </el-menu-item>
        <!-- 保留原有搜索菜单（折叠） -->
        <el-sub-menu index="search">
          <template #title>
            <el-icon><Search /></el-icon>
            <span>高级搜索</span>
          </template>
          <el-menu-item index="/search/resource">资源搜索框</el-menu-item>
          <el-menu-item index="/search/path">路径搜索框</el-menu-item>
          <el-menu-item index="/search/log">日志搜索框</el-menu-item>
          <el-menu-item index="/search/metrics">指标搜索框</el-menu-item>
          <el-menu-item index="/search/snapshot">搜索快照</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="views">
          <template #title>
            <el-icon><DataAnalysis /></el-icon>
            <span>视图列表</span>
          </template>
          <el-menu-item index="/views/list">视图列表</el-menu-item>
          <el-menu-item index="/views/detail">视图管理</el-menu-item>
          <el-menu-item index="/views/add-chart">添加图表</el-menu-item>
          <el-menu-item index="/views/variable-template">变量模板</el-menu-item>
        </el-sub-menu>
        <el-menu-item index="/business">
          <el-icon><Operation /></el-icon>
          <template #title>业务观测</template>
        </el-menu-item>
        <el-sub-menu index="app">
            <template #title>
              <el-icon><Monitor /></el-icon>
              <span>应用性能</span>
            </template>
            <el-menu-item index="/app/resource">资源分析</el-menu-item>
            <el-menu-item index="/app/path">路径分析</el-menu-item>
            <el-menu-item index="/app/topology">拓扑分析</el-menu-item>
            <el-menu-item index="/app/log">应用日志</el-menu-item>
            <el-menu-item index="/app/tracing">调用链分析</el-menu-item>
            <el-menu-item index="/app/file">文件分析</el-menu-item>
            <el-menu-item index="/app/drawer">右滑框</el-menu-item>
          </el-sub-menu>
        <el-menu-item index="/profiling">
          <el-icon><Cpu /></el-icon>
          <template #title>代码观测</template>
        </el-menu-item>
        <el-sub-menu index="network">
            <template #title>
              <el-icon><Connection /></el-icon>
              <span>基础设施监控</span>
            </template>
            <el-menu-item index="/network/resource">资源分析</el-menu-item>
            <el-menu-item index="/network/path">路径分析</el-menu-item>
            <el-menu-item index="/network/topology">拓扑分析</el-menu-item>
            <el-menu-item index="/network/flow">流日志</el-menu-item>
            <el-menu-item index="/network/nat">NAT追踪</el-menu-item>
            <el-menu-item index="/network/pcap">PCAP策略</el-menu-item>
            <el-menu-item index="/network/pcap-download">PCAP下载</el-menu-item>
            <el-menu-item index="/network/distribution">流量分发</el-menu-item>
            <el-menu-item index="/network/inventory">资源盘点</el-menu-item>
          </el-sub-menu>
        <el-sub-menu index="metrics-center">
            <template #title>
              <el-icon><DataAnalysis /></el-icon>
              <span>指标中心</span>
            </template>
            <el-menu-item index="/metrics-center/host">主机</el-menu-item>
            <el-menu-item index="/metrics-center/container">容器</el-menu-item>
            <el-menu-item index="/metrics-center/view">指标查看器</el-menu-item>
            <el-menu-item index="/metrics-center/summary">指标摘要</el-menu-item>
            <el-menu-item index="/metrics-center/template">指标模板</el-menu-item>
          </el-sub-menu>
        <el-sub-menu index="/log">
          <template #title>
            <el-icon><Message /></el-icon>
            <span>日志中心</span>
          </template>
          <el-menu-item index="/log">日志</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="/alert">
          <template #title>
            <el-icon><Warning /></el-icon>
            <span>告警管理</span>
          </template>
          <el-menu-item index="/alert/strategy">告警策略</el-menu-item>
          <el-menu-item index="/alert/endpoint">推送端点</el-menu-item>
          <el-menu-item index="/alert/event">告警事件</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="/report">
          <template #title>
            <el-icon><Document /></el-icon>
            <span>报表管理</span>
          </template>
          <el-menu-item index="/report/strategy">报表策略</el-menu-item>
          <el-menu-item index="/report/download">报表下载</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="/asset">
          <template #title>
            <el-icon><DataAnalysis /></el-icon>
            <span>资产列表</span>
          </template>
          <el-menu-item index="/asset/change-event">变更事件</el-menu-item>
          <el-menu-item index="/asset/resource-pool">资源池</el-menu-item>
          <el-menu-item index="/asset/compute">计算资源</el-menu-item>
          <el-menu-item index="/asset/network">网络资源</el-menu-item>
          <el-menu-item index="/asset/network-service">网络服务</el-menu-item>
          <el-menu-item index="/asset/storage">存储管理</el-menu-item>
          <el-menu-item index="/asset/container">容器资源</el-menu-item>
          <el-menu-item index="/asset/process">进程资源</el-menu-item>
          <el-menu-item index="/asset/other">其他资源</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="/system">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>系统管理</span>
          </template>
          <el-menu-item index="/system/collector">采集器</el-menu-item>
          <el-menu-item index="/system/data-node">数据节点管理</el-menu-item>
          <el-menu-item index="/system/account">账号与权限</el-menu-item>
          <el-menu-item index="/system/log">操作日志</el-menu-item>
        </el-sub-menu>
        <el-menu-item index="/profile">
          <el-icon><User /></el-icon>
          <template #title>个人设置</template>
        </el-menu-item>
      </el-menu>
      <div class="aside-footer">
        <div class="user-info">
          <el-avatar size="small">{{ userStore.userInfo?.username?.charAt(0)?.toUpperCase() || 'A' }}</el-avatar>
          <span class="username">{{ userStore.userInfo?.username || 'admin' }}</span>
          <el-button type="primary" link size="small" @click="handleLogout" style="margin-left: 8px;">退出</el-button>
        </div>
        <div class="copyright">
          © {{ currentYear }} Cloud Flow
        </div>
      </div>
    </el-aside>

 <!-- 主内容区 -->
    <div class="layout-main">
 <!-- 顶部导航栏 -->
      <el-header class="layout-header">
        <div class="header-left">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/' }">管理后台</el-breadcrumb-item>
            <el-breadcrumb-item v-for="(item, index) in breadcrumb" :key="index">
              {{ item }}
            </el-breadcrumb-item>
          </el-breadcrumb>
          <h2 class="page-title">{{ pageTitle }}</h2>
        </div>
      </el-header>

 <!-- 内部线程管理 -->
      <el-main class="layout-content">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '../stores/user'
import { DataAnalysis, Cpu, Setting, Document, Warning, Message, Connection, Operation, Monitor, Search, User } from '@element-plus/icons-vue'

const currentYear = new Date().getFullYear()

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const activeMenu = computed(() => {
  return route.path
})

const breadcrumb = ref<string[]>([])
const pageTitle = ref('')

watch(() => route.path, () => {
  const matched = route.matched.filter(item => item.meta && item.meta.title)
  breadcrumb.value = matched.map(item => (item.meta.title as string) || '')
  const lastMatched = matched[matched.length - 1]
  pageTitle.value = lastMatched ? (lastMatched.meta.title as string) || '默认页面' : '默认页面'
}, { immediate: true })

const handleLogout = () => {
  ElMessageBox.confirm('确定要退出登录吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    userStore.logout()
    router.push('/login')
    ElMessage.success('已退出登录')
  }).catch(() => {})
}
</script>

<style scoped>
.layout-container {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

.layout-aside {
  background-color: #001529;
  color: #fff;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

.aside-header {
  padding: 20px;
  border-bottom: 1px solid #002140;
}

.logo {
  font-size: 18px;
  font-weight: bold;
  margin: 0;
}

.aside-menu {
  flex: 1;
  border-right: none;
}

.aside-footer {
  padding: 20px;
  border-top: 1px solid #002140;
}

.user-info {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
}

.username {
  margin-left: 10px;
  font-size: 14px;
}

.copyright {
  font-size: 12px;
  color: #8c8c8c;
  text-align: center;
}

.layout-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.layout-header {
  background-color: #fff;
  border-bottom: 1px solid #e4e7ed;
  padding: 0 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 20px;
}

.page-title {
  font-size: 18px;
  font-weight: bold;
  margin: 0;
}

.layout-content {
  flex: 1;
  padding: 20px;
  background-color: #f5f7fa;
  overflow-y: auto;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>