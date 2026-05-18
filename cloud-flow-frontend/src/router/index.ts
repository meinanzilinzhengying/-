import { createRouter, createWebHistory } from 'vue-router'
import Layout from '../components/Layout.vue'
import NotFound from '../pages/NotFound.vue'
import { api } from '../utils/api'

const routes = [
 // 登录页面
  {
    path: '/login',
    name: 'Login',
    component: () => import('../pages/Login.vue'),
    meta: { title: '登录' }
  },
 // 根路径重定向
  {
    path: '/',
    redirect: '/dashboard'
  },
  {
    path: '/',
    component: Layout,
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('../pages/Dashboard.vue'),
        meta: { title: '仪表盘' }
      },

      {
        path: 'asset',
        component: () => import('../pages/Asset.vue'),
        children: [
          {
            path: 'change-event',
            name: 'ChangeEvent',
            component: () => import('../components/asset/ChangeEvent.vue'),
            meta: { title: '变更事件' }
          },
          {
            path: 'resource-pool',
            name: 'ResourcePool',
            component: () => import('../components/asset/ResourcePool.vue'),
            meta: { title: '资源池' }
          },
          {
            path: 'compute',
            name: 'Compute',
            component: () => import('../components/asset/ComputeResource.vue'),
            meta: { title: '计算资源' }
          },
          {
            path: 'network',
            name: 'Network',
            component: () => import('../components/asset/NetworkResource.vue'),
            meta: { title: '网络资源' }
          },
          {
            path: 'network-service',
            name: 'NetworkService',
            component: () => import('../components/asset/NetworkService.vue'),
            meta: { title: '网络服务' }
          },
          {
            path: 'storage',
            name: 'Storage',
            component: () => import('../components/asset/StorageResource.vue'),
            meta: { title: '存储服务' }
          },
          {
            path: 'container',
            name: 'Container',
            component: () => import('../components/asset/ContainerResource.vue'),
            meta: { title: '容器资源' }
          },
          {
            path: 'process',
            name: 'Process',
            component: () => import('../components/asset/ProcessResource.vue'),
            meta: { title: '进程资源' }
          },
          {
            path: 'other',
            name: 'Other',
            component: () => import('../components/asset/OtherResource.vue'),
            meta: { title: '其他资源' }
          }
        ]
      },
      {
        path: 'system',
        component: () => import('../pages/System.vue'),
        children: [
          {
            path: 'collector',
            name: 'Collector',
            component: () => import('../pages/system/CollectorManagement.vue'),
            meta: { title: '采集器' }
          },
          {
            path: 'data-node',
            name: 'DataNode',
            component: () => import('../pages/system/DataNodeManagement.vue'),
            meta: { title: '数据库节点' }
          },
          {
            path: 'account',
            name: 'Account',
            component: () => import('../pages/system/AccountManagement.vue'),
            meta: { title: '账号管理' }
          },
          {
            path: 'log',
            name: 'Log',
            component: () => import('../pages/system/SystemLog.vue'),
            meta: { title: '操作日志' }
          }
        ]
      },
      {
        path: 'report',
        component: () => import('../pages/Report.vue'),
        children: [
          {
            path: 'strategy',
            name: 'ReportStrategy',
            component: () => import('../pages/ReportStrategy.vue'),
            meta: { title: '报表策略', reportType: 'strategy' }
          },
          {
            path: 'download',
            name: 'ReportDownload',
            component: () => import('../pages/ReportDownload.vue'),
            meta: { title: '报表下载', reportType: 'download' }
          }
        ]
      },
      {
        path: 'alert',
        component: () => import('../pages/Alert.vue'),
        children: [
          {
            path: 'strategy',
            name: 'AlertStrategy',
            component: () => import('../pages/AlertStrategy.vue'),
            meta: { title: '告警策略', alertType: 'strategy' }
          },
          {
            path: 'endpoint',
            name: 'AlertEndpoint',
            component: () => import('../pages/AlertEndpoint.vue'),
            meta: { title: '推送端点', alertType: 'endpoint' }
          },
          {
            path: 'event',
            name: 'AlertEvent',
            component: () => import('../pages/AlertEvent.vue'),
            meta: { title: '告警事件', alertType: 'event' }
          }
        ]
      },
      {
        path: 'log',
        name: 'Log',
        component: () => import('../pages/Log.vue'),
        meta: { title: '日志' }
      },
      {
        path: 'metrics-center',
        component: () => import('../pages/MetricsCenter.vue'),
        children: [
          {
            path: 'host',
            name: 'MetricsCenterHost',
            component: () => import('../pages/metrics/HostMetrics.vue'),
            meta: { title: '主机', metricsType: 'host' }
          },
          {
            path: 'container',
            name: 'MetricsCenterContainer',
            component: () => import('../pages/metrics/ContainerMetrics.vue'),
            meta: { title: '容器', metricsType: 'container' }
          },
          {
            path: 'view',
            name: 'MetricsCenterView',
            component: () => import('../pages/metrics/MetricsView.vue'),
            meta: { title: '指标查看', metricsType: 'view' }
          },
          {
            path: 'summary',
            name: 'MetricsCenterSummary',
            component: () => import('../pages/metrics/MetricsSummary.vue'),
            meta: { title: '指标摘要', metricsType: 'summary' }
          },
          {
            path: 'template',
            name: 'MetricsCenterTemplate',
            component: () => import('../pages/metrics/MetricsTemplate.vue'),
            meta: { title: '指标模板', metricsType: 'template' }
          }
        ]
      },
      {
        path: 'network',
        component: () => import('../pages/Network.vue'),
        children: [
          {
            path: 'resource',
            name: 'NetworkResource',
            component: () => import('../components/network/ResourceAnalysis.vue'),
            meta: { title: '资源分析', networkType: 'resource' }
          },
          {
            path: 'path',
            name: 'NetworkPath',
            component: () => import('../components/network/PathAnalysis.vue'),
            meta: { title: '路径分析', networkType: 'path' }
          },
          {
            path: 'topology',
            name: 'NetworkTopology',
            component: () => import('../components/network/TopologyAnalysis.vue'),
            meta: { title: '拓扑分析', networkType: 'topology' }
          },
          {
            path: 'flow',
            name: 'NetworkFlow',
            component: () => import('../components/network/FlowAnalysis.vue'),
            meta: { title: '流日志', networkType: 'flow' }
          },
          {
            path: 'nat',
            name: 'NetworkNAT',
            component: () => import('../components/network/NATAnalysis.vue'),
            meta: { title: 'NAT追踪', networkType: 'nat' }
          },
          {
            path: 'pcap',
            name: 'NetworkPCAP',
            component: () => import('../components/network/PCAPStrategy.vue'),
            meta: { title: 'PCAP策略', networkType: 'pcap' }
          },
          {
            path: 'pcap-download',
            name: 'NetworkPCAPDownload',
            component: () => import('../components/network/PCAPDownload.vue'),
            meta: { title: 'PCAP下载', networkType: 'pcap-download' }
          },
          {
            path: 'distribution',
            name: 'NetworkDistribution',
            component: () => import('../components/network/FlowDistribution.vue'),
            meta: { title: '流量分发', networkType: 'distribution' }
          },
          {
            path: 'inventory',
            name: 'NetworkInventory',
            component: () => import('../components/network/ResourceInventory.vue'),
            meta: { title: '资源盘点', networkType: 'inventory' }
          }
        ]
      },
      {
        path: 'profiling',
        name: 'Profiling',
        component: () => import('../pages/Profiling.vue'),
        meta: { title: '代码观测' }
      },
      {
        path: 'business',
        component: () => import('../pages/Business.vue'),
        meta: { title: '业务观测' },
        children: [
          {
            path: '',
            name: 'BusinessHome',
            redirect: '/business/definition'
          },
          {
            path: 'definition',
            name: 'BusinessDefinition',
            component: () => import('../pages/BusinessDefinition.vue'),
            meta: { title: '业务定义' }
          },
          {
            path: 'detail/:id',
            name: 'BusinessDetail',
            component: () => import('../pages/BusinessDetail.vue'),
            meta: { title: '业务详情' }
          },
          {
            path: 'topology/:id',
            name: 'BusinessTopologyView',
            component: () => import('../pages/BusinessTopologyView.vue'),
            meta: { title: '服务拓扑' }
          },
          {
            path: 'services/:id',
            name: 'BusinessServicesView',
            component: () => import('../pages/BusinessServicesView.vue'),
            meta: { title: '服务列表' }
          }
        ]
      },
      {
        path: 'service-list',
        name: 'ServiceList',
        component: () => import('../pages/ServiceList.vue'),
        meta: { title: '服务列表' }
      },
      {
        path: 'service-topology',
        name: 'ServiceTopology',
        component: () => import('../pages/ServiceTopology.vue'),
        meta: { title: '服务拓扑' }
      },
      {
        path: 'views',
        component: () => import('../pages/Views.vue'),
        meta: { title: '视图列表' },
        children: [
          {
            path: 'list',
            name: 'ViewList',
            component: () => import('../pages/ViewList.vue'),
            meta: { title: '视图列表' }
          },
          {
            path: 'detail',
            name: 'ViewDetail',
            component: () => import('../pages/ViewDetail.vue'),
            meta: { title: '视图详情' }
          },
          {
            path: 'add-chart',
            name: 'AddChart',
            component: () => import('../pages/AddChart.vue'),
            meta: { title: '添加图表' }
          },
          {
            path: 'variable-template',
            name: 'VariableTemplate',
            component: () => import('../pages/VariableTemplate.vue'),
            meta: { title: '变量模板' }
          }
        ]
      },
      {
        path: 'app',
        component: () => import('../pages/App.vue'),
        children: [
          {
            path: 'resource',
            name: 'AppResource',
            component: () => import('../pages/AppResource.vue'),
            meta: { title: '资源分析' }
          },
          {
            path: 'path',
            name: 'AppPath',
            component: () => import('../pages/AppPath.vue'),
            meta: { title: '路径分析' }
          },
          {
            path: 'topology',
            name: 'AppTopology',
            component: () => import('../pages/AppTopology.vue'),
            meta: { title: '拓扑分析' }
          },
          {
            path: 'log',
            name: 'AppLog',
            component: () => import('../pages/AppLog.vue'),
            meta: { title: '调用日志' }
          },
          {
            path: 'tracing',
            name: 'AppTracing',
            component: () => import('../pages/AppTracing.vue'),
            meta: { title: '调用追踪' }
          },
          {
            path: 'file',
            name: 'AppFile',
            component: () => import('../pages/AppFile.vue'),
            meta: { title: '文件读写' }
          },
          {
            path: 'drawer',
            name: 'AppDrawer',
            component: () => import('../pages/AppDrawer.vue'),
            meta: { title: '右滑框' }
          }
        ]
      },

      // 聚合搜索中心 - 统一搜索入口
      {
        path: 'search-center',
        name: 'SearchCenter',
        component: () => import('../pages/SearchCenter.vue'),
        meta: { title: '搜索中心' }
      },
      // 保留原有搜索路由作为兼容
      {
        path: 'search',
        component: () => import('../views/Search.vue'),
        meta: { title: '数据库搜索' },
        children: [
          {
            path: 'resource',
            name: 'SearchResource',
            component: () => import('../pages/SearchResource.vue'),
            meta: { title: '资源搜索框' }
          },
          {
            path: 'path',
            name: 'SearchPath',
            component: () => import('../pages/SearchPath.vue'),
            meta: { title: '路径搜索框' }
          },
          {
            path: 'log',
            name: 'SearchLog',
            component: () => import('../pages/SearchLog.vue'),
            meta: { title: '日志搜索框' }
          },
          {
            path: 'metrics',
            name: 'SearchMetrics',
            component: () => import('../pages/SearchMetrics.vue'),
            meta: { title: '指标搜索框' }
          },
          {
            path: 'snapshot',
            name: 'SearchSnapshot',
            component: () => import('../pages/SearchSnapshot.vue'),
            meta: { title: '搜索快照' }
          },
        ]
      },
      // 聚合分析中心 - 统一分析入口
      {
        path: 'analysis-center',
        name: 'AnalysisCenter',
        component: () => import('../pages/AnalysisCenter.vue'),
        meta: { title: '分析中心' }
      },
      // 聚合日志中心 - 统一日志入口
      {
        path: 'log-center',
        name: 'LogCenter',
        component: () => import('../pages/LogCenter.vue'),
        meta: { title: '日志中心' }
      },
      {
        path: 'profile',
        name: 'Profile',
        component: () => import('../pages/Profile.vue'),
        meta: { title: '个人设置' }
      }
    ]
  },
 // 404页面
  {
    path: '/:pathMatch(.*)*',
    component: NotFound
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 认证状态缓存（带过期时间，避免 token 失效后缓存仍为 true）
let authCache: { valid: boolean; timestamp: number } | null = null;
const AUTH_CACHE_TTL = 5 * 60 * 1000; // 缓存有效期 5 分钟

// 路由守卫（未登录跳转）
router.beforeEach(async (to, _from, next) => {
  if (to.name !== 'Login') {
    // 检查缓存是否过期
    const now = Date.now();
    if (authCache !== null && (now - authCache.timestamp) < AUTH_CACHE_TTL) {
      if (!authCache.valid) {
        next({ name: 'Login' });
        return;
      }
      // 缓存有效且认证通过，放行
    } else {
      // 缓存过期或不存在，重新验证
      try {
        const isAuth = await api.isAuthenticated();
        authCache = { valid: isAuth, timestamp: now };
        if (!isAuth) {
          next({ name: 'Login' });
          return;
        }
      } catch (e) {
        // 验证失败，清除缓存并跳转到登录页
        authCache = null;
        next({ name: 'Login' });
        return;
      }
    }
  }
  next();
});

// 清除认证缓存的函数（用于401等情况）
export function clearAuthCache() {
  authCache = null;
}

export default router
