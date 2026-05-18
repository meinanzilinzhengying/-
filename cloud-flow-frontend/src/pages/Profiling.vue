<template>

  <div class="profiling-container">

 <!-- 顶部控制-->

    <div class="profiling-header">

      <div class="profiling-search">

        <el-form :inline="true" :model="profilingForm" class="demo-form-inline">

          <el-form-item label="搜索快照">

            <el-select v-model="profilingForm.snapshot" placeholder="查询快照" style="width: 200px;">

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

          <el-form-item>

            <el-input v-model="profilingForm.search" placeholder="输入搜索条件" style="width: 300px;" />

          </el-form-item>

          <el-form-item>

            <el-button type="primary" @click="searchProfiling">

              <el-icon><Search /></el-icon> 搜索

            </el-button>

          </el-form-item>

        </el-form>

      </div>

      <div class="profiling-actions">

        <el-form :inline="true" :model="profilingActionsForm" class="demo-form-inline">

          <el-form-item>

            <el-button @click="refreshProfiling">

              <el-icon><Refresh /></el-icon> 刷新

            </el-button>

          </el-form-item>

          <el-form-item>

            <el-button @click="exportProfiling">

              <el-icon><Download /></el-icon> 导出

            </el-button>

          </el-form-item>

        </el-form>

      </div>

    </div>

    

 <!-- 主要内容-->

    <div class="profiling-content">

 <!-- 左侧过滤面板 -->

      <div class="profiling-sidebar">

 <!-- 应用列表 -->

        <div class="filter-section">

          <h3>应用列表</h3>

          <el-checkbox-group v-model="selectedApps">

            <el-checkbox label="Total (eBPF)">Total (eBPF)</el-checkbox>

            <el-checkbox label="apiserver (eBPF)">apiserver (eBPF)</el-checkbox>

            <el-checkbox label="apiserver-ingress (eBPF)">apiserver-ingress (eBPF)</el-checkbox>

            <el-checkbox label="cartservice (eBPF)">cartservice (eBPF)</el-checkbox>

            <el-checkbox label="checkoutservice (eBPF)">checkoutservice (eBPF)</el-checkbox>

            <el-checkbox label="deepflow-agent (eBPF)" checked>deepflow-agent (eBPF)</el-checkbox>

            <el-checkbox label="deepflow-jacoco (eBPF)">deepflow-jacoco (eBPF)</el-checkbox>

            <el-checkbox label="etcd (eBPF)">etcd (eBPF)</el-checkbox>

            <el-checkbox label="etcdctl (eBPF)">etcdctl (eBPF)</el-checkbox>

          </el-checkbox-group>

        </div>

        

 <!-- 分析类型 -->

        <div class="filter-section">

          <h3>分析类型</h3>

          <el-radio-group v-model="profilingType">

            <el-radio label="on-cpu" checked>on-cpu</el-radio>

            <el-radio label="off-cpu">off-cpu</el-radio>

            <el-radio label="alloc">alloc</el-radio>

            <el-radio label="lock">lock</el-radio>

          </el-radio-group>

        </div>

      </div>

      

 <!-- 右侧内容区-->

      <div class="profiling-main">

 <!-- 趋势分析 -->

        <div class="trend-analysis">

          <h3>趋势分析：花费CPU的时间</h3>

          <div class="mock-chart trend-chart">

            <div class="chart-bars">

              <div v-for="i in 60" :key="i" class="chart-bar trend-bar" :style="{ height: trendChartData[i-1] + '%' }"></div>

            </div>

            <div class="chart-x-axis">

              <div v-for="i in 10" :key="i" class="x-axis-label">{{ 10 + i }}:24</div>

            </div>

          </div>

          <div class="chart-info">

            <span>语言类型=eBPF, 分析类型=on-cpu</span>

          </div>

        </div>

        

 <!-- 数据库过滤 -->

        <div class="data-filter">

          <el-input v-model="dataFilter" placeholder="名称" style="width: 200px;" />

        </div>

        

 <!-- 展示切换 -->

        <div class="display-toggle">

          <el-button-group>

            <el-button :type="displayMode === 'flame' ? 'primary' : ''" @click="displayMode = 'flame'">

              <el-icon><Grid /></el-icon> 火焰图

            </el-button>

            <el-button :type="displayMode === 'table' ? 'primary' : ''" @click="displayMode = 'table'">

              <el-icon><DataAnalysis /></el-icon> 列表

            </el-button>

            <el-button :type="displayMode === 'both' ? 'primary' : ''" @click="displayMode = 'both'">

              <el-icon><Monitor /></el-icon> 同时展示

            </el-button>

          </el-button-group>

        </div>

        

 <!-- 火焰图名称显示-->

        <div class="flame-name-toggle">

          <el-select v-model="flameNameDisplay" placeholder="火焰图名称显示" style="width: 150px;">

            <el-option label="头部" value="head" />

            <el-option label="尾部" value="tail" />

          </el-select>

        </div>

        

 <!-- 内容区域 -->

        <div class="content-area">

 <!-- 火焰图-->

          <div v-if="displayMode === 'flame' || displayMode === 'both'" class="flame-chart-container">

            <h3>deepflow-agent (188.86%, 21.584m) 每个函数花费 CPU 的时间</h3>

            <div class="mock-flame-chart">

              <div class="flame-row">

                <div class="flame-cell kernel" style="width: 10%;">

                  <div class="flame-tooltip">

                    <div>函数类型: K (Linux 内核函数)</div>

                    <div>Span 名称: cp_reader_work</div>

                    <div>总消耗 6.68% (1.441m)</div>

                    <div>自身消耗 6.68% (1.441m)</div>

                  </div>

                  <span>cp_reader_work</span>

                </div>

                <div class="flame-cell lib" style="width: 15%;">

                  <div class="flame-tooltip">

                    <div>函数类型: L (动态链接库)</div>

                    <div>Span 名称: __libc_malloc</div>

                    <div>总消耗 3.81% (0.822m)</div>

                    <div>自身消耗 3.81% (0.822m)</div>

                  </div>

                  <span>__libc_malloc</span>

                </div>

                <div class="flame-cell app" style="width: 25%;">

                  <div class="flame-tooltip">

                    <div>函数类型: A (应用程序)</div>

                    <div>Span 名称: resolve_stack</div>

                    <div>总消耗 12.36% (2.663m)</div>

                    <div>自身消耗 12.36% (2.663m)</div>

                  </div>

                  <span>resolve_stack</span>

                </div>

                <div class="flame-cell process" style="width: 10%;">

                  <div class="flame-tooltip">

                    <div>函数类型: P (进程)</div>

                    <div>Span 名称: build_stack</div>

                    <div>总消耗 5.76% (1.243m)</div>

                    <div>自身消耗 5.76% (1.243m)</div>

                  </div>

                  <span>build_stack</span>

                </div>

                <div class="flame-cell thread" style="width: 8%;">

                  <div class="flame-tooltip">

                    <div>函数类型: T (线程)</div>

                    <div>Span 名称: socket_recv</div>

                    <div>总消耗 2.18% (0.470m)</div>

                    <div>自身消耗 2.18% (0.470m)</div>

                  </div>

                  <span>socket_recv</span>

                </div>

                <div class="flame-cell unknown" style="width: 32%;">

                  <div class="flame-tooltip">

                    <div>函数类型: ? (未知)</div>

                    <div>Span 名称: do_syscall_64</div>

                    <div>总消耗 11.29% (2.437m)</div>

                    <div>自身消耗 11.29% (2.437m)</div>

                  </div>

                  <span>do_syscall_64</span>

                </div>

              </div>

              <div class="flame-row">

                <div class="flame-cell kernel" style="width: 12%;">

                  <div class="flame-tooltip">

                    <div>函数类型: K (Linux 内核函数)</div>

                    <div>Span 名称: established_get_first_ira</div>

                    <div>总消耗 2.54% (0.548m)</div>

                    <div>自身消耗 2.54% (0.548m)</div>

                  </div>

                  <span>established_get_first_ira</span>

                </div>

                <div class="flame-cell lib" style="width: 18%;">

                  <div class="flame-tooltip">

                    <div>函数类型: L (动态链接库)</div>

                    <div>Span 名称: __GI___getpwnam_r</div>

                    <div>总消耗 3.98% (0.860m)</div>

                    <div>自身消耗 3.98% (0.860m)</div>

                  </div>

                  <span>__GI___getpwnam_r</span>

                </div>

                <div class="flame-cell app" style="width: 22%;">

                  <div class="flame-tooltip">

                    <div>函数类型: A (应用程序)</div>

                    <div>Span 名称: reader_raw_cb</div>

                    <div>总消耗 8.75% (1.889m)</div>

                    <div>自身消耗 8.75% (1.889m)</div>

                  </div>

                  <span>reader_raw_cb</span>

                </div>

                <div class="flame-cell process" style="width: 15%;">

                  <div class="flame-tooltip">

                    <div>函数类型: P (进程)</div>

                    <div>Span 名称: build_stack_trace_string_ira_5_const</div>

                    <div>总消耗 3.23% (0.696m)</div>

                    <div>自身消耗 3.23% (0.696m)</div>

                  </div>

                  <span>build_stack_trace_string_ira_5_const</span>

                </div>

                <div class="flame-cell thread" style="width: 10%;">

                  <div class="flame-tooltip">

                    <div>函数类型: T (线程)</div>

                    <div>Span 名称: core_syscall</div>

                    <div>总消耗 1.97% (0.426m)</div>

                    <div>自身消耗 1.97% (0.426m)</div>

                  </div>

                  <span>core_syscall</span>

                </div>

                <div class="flame-cell unknown" style="width: 23%;">

                  <div class="flame-tooltip">

                    <div>函数类型: ? (未知)</div>

                    <div>Span 名称: __futex_wait</div>

                    <div>总消耗 5.12% (1.105m)</div>

                    <div>自身消耗 5.12% (1.105m)</div>

                  </div>

                  <span>__futex_wait</span>

                </div>

              </div>

            </div>

          </div>

          

 <!-- 表格 -->

          <div v-if="displayMode === 'table' || displayMode === 'both'" class="table-container">

            <el-table :data="profilingData" style="width: 100%">

              <el-table-column prop="name" label="名称" width="300" />

              <el-table-column prop="selfTime" label="自身消耗" width="150" />

              <el-table-column prop="totalTime" label="总消耗" width="150" />

            </el-table>

          </div>

        </div>

      </div>

    </div>

  </div>

</template>



<script setup lang="ts">

// 生成模拟数据库（仅在组件挂载时调用一次，避免图表跳动）
const generateMockData = (max: number, min: number, count: number = 30) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Refresh, Download, Grid, DataAnalysis, Monitor } from '@element-plus/icons-vue'



// 表单数据库

const profilingForm = ref({

  snapshot: '15m',

  search: ''

})



const profilingActionsForm = ref({})



// 应用列表

const selectedApps = ref(['deepflow-agent (eBPF)'])



// 分析类型

const profilingType = ref('on-cpu')



// 展示模式

const displayMode = ref('both')



// 火焰图名称显示

const flameNameDisplay = ref('head')



// 数据库过滤

const dataFilter = ref('')



// 趋势图表数据库

const trendChartData = ref([])

onMounted(() => {
  trendChartData.value = generateMockData(80, 20, 60)
})




// 分析数据库

const profilingData = ref([

  {

    name: '[/usr/lib/x86_64-linux-gnu/libc.so.6]',

    selfTime: '2.77ms',

    totalTime: '13.36ms'

  },

  {

    name: '[k] established_get_first_ira',

    selfTime: '1.44ms',

    totalTime: '1.47ms'

  },

  {

    name: '[k] established_get_first_ira.37',

    selfTime: '48.83s',

    totalTime: '54.2s'

  },

  {

    name: '[l] __libc_malloc',

    selfTime: '38.51s',

    totalTime: '39.81s'

  },

  {

    name: '[k] __spin_unlock_irqrestore',

    selfTime: '37.14s',

    totalTime: '37.84s'

  },

  {

    name: '[k] finish_task_switch',

    selfTime: '28.35s',

    totalTime: '28.44s'

  },

  {

    name: '[A] cp_reader_work',

    selfTime: '27.52s',

    totalTime: '8.823s'

  },

  {

    name: '[A] ProcSys::Module::find_addr(unsigned int)',

    selfTime: '24.12s',

    totalTime: '25.66s'

  },

  {

    name: '[k] rr_block_pa_pages.lrqrestore',

    selfTime: '23.42s',

    totalTime: '24.64s'

  },

  {

    name: '[k] __get_user',

    selfTime: '19.97s',

    totalTime: '19.99s'

  }

])



// 搜索剖析
const searchProfiling = () => {
  ElMessage.info('功能开发中...')
}

// 刷新分析
const refreshProfiling = () => {
  ElMessage.info('功能开发中...')
}

// 导出剖析
const exportProfiling = () => {
  ElMessage.info('功能开发中...')
}

</script>



<style scoped>

.profiling-container {

  padding: 20px;

}



.profiling-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-bottom: 20px;

  padding: 15px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.profiling-search {

  flex: 1;

}



.profiling-actions {

  display: flex;

  align-items: center;

  gap: 10px;

}



.profiling-content {

  display: flex;

  gap: 20px;

}



.profiling-sidebar {

  width: 250px;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.filter-section {

  margin-bottom: 20px;

}



.filter-section h3 {

  margin-top: 0;

  margin-bottom: 10px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.profiling-main {

  flex: 1;

  background-color: white;

  border-radius: 4px;

  padding: 15px;

}



.trend-analysis {

  margin-bottom: 30px;

}



.trend-analysis h3 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 16px;

  font-weight: bold;

  color: #303133;

}



.trend-chart {

  height: 200px;

  margin-bottom: 10px;

}



.trend-bar {

  background-color: #67c23a;

  border-radius: 2px 2px 0 0;

}



.chart-info {

  font-size: 12px;

  color: #909399;

}



.data-filter {

  margin-bottom: 15px;

}



.display-toggle {

  margin-bottom: 15px;

}



.flame-name-toggle {

  margin-bottom: 20px;

}



.content-area {

  display: flex;

  flex-direction: column;

  gap: 20px;

}



.flame-chart-container {

  background-color: #f5f7fa;

  border-radius: 4px;

  padding: 15px;

}



.flame-chart-container h3 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.mock-flame-chart {

  background-color: #1e1e1e;

  border-radius: 4px;

  padding: 10px;

  color: white;

  font-size: 12px;

}



.flame-row {

  display: flex;

  height: 40px;

  margin-bottom: 5px;

}



.flame-cell {

  display: flex;

  align-items: center;

  justify-content: center;

  border-radius: 2px;

  position: relative;

  cursor: pointer;

  overflow: hidden;

}



.flame-cell:hover .flame-tooltip {

  display: block;

}



.flame-tooltip {

  display: none;

  position: absolute;

  top: -100px;

  left: 0;

  background-color: rgba(0, 0, 0, 0.8);

  color: white;

  padding: 10px;

  border-radius: 4px;

  z-index: 100;

  width: 200px;

  font-size: 12px;

}



.flame-cell.kernel {

  background-color: #8884d8;

}



.flame-cell.lib {

  background-color: #82ca9d;

}



.flame-cell.app {

  background-color: #ffc658;

}



.flame-cell.process {

  background-color: #ff8042;

}



.flame-cell.thread {

  background-color: #0088fe;

}



.flame-cell.unknown {

  background-color: #00c49f;

}



.table-container {

  background-color: #f5f7fa;

  border-radius: 4px;

  padding: 15px;

}



.table-container .el-table {

  margin-bottom: 0;

}



@media (min-width: 1200px) {

  .content-area {

    flex-direction: row;

  }

  

  .flame-chart-container {

    flex: 1;

  }

  

  .table-container {

    flex: 1;

  }

}

</style>