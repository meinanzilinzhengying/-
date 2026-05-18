<template>

  <div class="app-drawer-content">

    <h2>抽屉功能演示</h2>

    <p>抽屉功能已集成到各个页面中，点击拓扑图、折线图、柱状图等元素即可调出相关面板</p>

    <p>抽屉支持以下功能：</p>

    <ul>

      <li>知识图谱：展示点击对象的关联标签(Tag)</li>

      <li>访问关系：展示点击对象的上下调用指标</li>

      <li>应用指标：显示一段时间内应用指标变化趋势</li>

      <li>端点列表：展示服务分组的指标</li>

      <li>调用日志：展示点击数据库的详细调用时间</li>

      <li>调用链追踪：展示请求的详细调用链路信息</li>

      <li>网络指标：展示网络指标的变化趋势</li>

      <li>流日志详情：展示流日志详细信息</li>

      <li>NAT 映射：三种路径模式下的 NAT 前后流量</li>

      <li>事件：展示资源变更事件和文件读写事件</li>

    </ul>

    <div class="demo-button">

      <el-button type="primary" @click="openDrawer">打开抽屉功能演示</el-button>

    </div>

    

 <!-- 抽屉功能演示-->

    <el-drawer

      v-model="drawerVisible"

      title="抽屉功能演示"

      direction="rtl"

      size="50%"

    >

      <div class="drawer-demo">

        <el-tabs v-model="activeTab">

          <el-tab-pane label="知识图谱" name="knowledge">

            <div class="knowledge-graph">

              <h3>知识图谱</h3>

              <div class="knowledge-tabs">

                <el-tabs v-model="knowledgeTab">

                  <el-tab-pane label="知识列表" name="list">

                    <div class="knowledge-list">

                      <div class="knowledge-search">

                        <el-input v-model="knowledgeSearch" placeholder="搜索标签" style="width: 300px;" />

                      </div>

                      <div class="knowledge-filters">

                        <el-checkbox-group v-model="knowledgeFilters">

                          <el-checkbox label="Client Universal Tag">Client Universal Tag</el-checkbox>

                          <el-checkbox label="Server Universal Tag">Server Universal Tag</el-checkbox>

                          <el-checkbox label="Client Custom Tag">Client Custom Tag</el-checkbox>

                          <el-checkbox label="Server Custom Tag">Server Custom Tag</el-checkbox>

                          <el-checkbox label="Others">Others</el-checkbox>

                        </el-checkbox-group>

                      </div>

                      <div class="knowledge-content">

                        <h4>Client Universal Tag</h4>

                        <div class="tag-list">

                          <div class="tag-item">

                            <span class="tag-key">region (区域):</span>

                            <span class="tag-value">9-21-4-23</span>

                            <el-button size="small" @click="copyTag('region', '9-21-4-23')">复制</el-button>

                          </div>

                          <div class="tag-item">

                            <span class="tag-key">az (可用区:</span>

                            <span class="tag-value">T0-Sandbox</span>

                            <el-button size="small" @click="copyTag('az', 'T0-Sandbox')">复制</el-button>

                          </div>

                          <div class="tag-item">

                            <span class="tag-key">host (主机名称):</span>

                            <span class="tag-value">c7i-00b1</span>

                            <el-button size="small" @click="copyTag('host', 'c7i-00b1')">复制</el-button>

                          </div>

                        </div>

                        <h4>Server Universal Tag</h4>

                        <div class="tag-list">

                          <div class="tag-item">

                            <span class="tag-key">region (区域):</span>

                            <span class="tag-value">9-21-4-23</span>

                            <el-button size="small" @click="copyTag('region', '9-21-4-23')">复制</el-button>

                          </div>

                          <div class="tag-item">

                            <span class="tag-key">az (可用区:</span>

                            <span class="tag-value">T0-Sandbox</span>

                            <el-button size="small" @click="copyTag('az', 'T0-Sandbox')">复制</el-button>

                          </div>

                          <div class="tag-item">

                            <span class="tag-key">host (主机名称):</span>

                            <span class="tag-value">c7i-00b2</span>

                            <el-button size="small" @click="copyTag('host', 'c7i-00b2')">复制</el-button>

                          </div>

                        </div>

                      </div>

                    </div>

                  </el-tab-pane>

                  <el-tab-pane label="知识图谱" name="graph">

                    <div class="knowledge-graph-content">

                      <div class="mock-graph">

                        <div class="graph-node center-node">

                          <div class="node-content">

                            <div class="node-label">web-shop</div>

                          </div>

                        </div>

                        <div class="graph-node left-node">

                          <div class="node-content">

                            <div class="node-label">Client</div>

                          </div>

                        </div>

                        <div class="graph-node right-node">

                          <div class="node-content">

                            <div class="node-label">Server</div>

                          </div>

                        </div>

                        <div class="graph-node bottom-node">

                          <div class="node-content">

                            <div class="node-label">Others</div>

                          </div>

                        </div>

                        <svg class="graph-connections" width="100%" height="100%">

                          <line x1="400" y1="150" x2="200" y2="250" stroke="#409eff" stroke-width="2" />

                          <line x1="400" y1="150" x2="600" y2="250" stroke="#409eff" stroke-width="2" />

                          <line x1="400" y1="150" x2="400" y2="350" stroke="#409eff" stroke-width="2" />

                        </svg>

                      </div>

                    </div>

                  </el-tab-pane>

                </el-tabs>

              </div>

            </div>

          </el-tab-pane>

          <el-tab-pane label="访问关系拓扑" name="access">

            <div class="access-relation">

              <h3>访问关系拓扑</h3>

              <div class="access-controls">

                <el-form :inline="true" :model="accessForm" class="demo-form-inline">

                  <el-form-item>

                    <el-select v-model="accessForm.service" placeholder="选择服务" style="width: 200px;">

                      <el-option label="web-shop" value="web-shop" />

                      <el-option label="otel-agent" value="otel-agent" />

                      <el-option label="svc-user" value="svc-user" />

                    </el-select>

                  </el-form-item>

                  <el-form-item>

                    <el-checkbox-group v-model="accessForm.roles">

                      <el-checkbox label="作为客户端服务">作为客户端服务</el-checkbox>

                      <el-checkbox label="作为服务端服务">作为服务端服务</el-checkbox>

                    </el-checkbox-group>

                  </el-form-item>

                </el-form>

              </div>

              <div class="access-table">

                <h4>流量统计</h4>

                <el-table :data="accessData" style="width: 100%" @row-click="handleAccessRowClick">

                  <el-table-column prop="client" label="客户端服务" width="180" />

                  <el-table-column prop="server" label="服务端列表" width="180" />

                  <el-table-column prop="group" label="服务实例分组" width="150" />

                  <el-table-column prop="requestRate" label="网络流量监控" width="100" />

                  <el-table-column prop="errorRate" label="服务端错误率" width="100" />

                  <el-table-column prop="responseTime" label="响应时间" width="100" />

                </el-table>

              </div>

              <div class="access-topology">

                <h4>访问关系拓扑</h4>

                <el-tabs v-model="accessTopologyTab">

                  <el-tab-pane label="访问关系拓扑" name="topology">

                    <div class="topology-content">

                      <div class="mock-topology">

                        <div class="topology-node">

                          <div class="node-content">

                            <div class="node-icon">进程</div>

                            <div class="node-label">进程/容器</div>

                          </div>

                        </div>

                        <div class="topology-node">

                          <div class="node-content">

                            <div class="node-icon">网络</div>

                            <div class="node-label">进程/容器/服务</div>

                          </div>

                        </div>

                        <div class="topology-node">

                          <div class="node-content">

                            <div class="node-icon">容器</div>

                            <div class="node-label">客户端错误率管理</div>

                          </div>

                        </div>

                        <div class="topology-node">

                          <div class="node-content">

                            <div class="node-icon">容器</div>

                            <div class="node-label">服务端容器实例</div>

                          </div>

                        </div>

                        <div class="topology-node">

                          <div class="node-content">

                            <div class="node-icon">网络</div>

                            <div class="node-label">默认网关</div>

                          </div>

                        </div>

                        <div class="topology-node">

                          <div class="node-content">

                            <div class="node-icon">进程</div>

                            <div class="node-label">默认网络</div>

                          </div>

                        </div>

                        <svg class="topology-connections" width="100%" height="100%">

                          <line x1="100" y1="150" x2="200" y2="150" stroke="#409eff" stroke-width="2" />

                          <line x1="200" y1="150" x2="300" y2="150" stroke="#409eff" stroke-width="2" />

                          <line x1="300" y1="150" x2="400" y2="150" stroke="#409eff" stroke-width="2" />

                          <line x1="400" y1="150" x2="500" y2="150" stroke="#409eff" stroke-width="2" />

                          <line x1="500" y1="150" x2="600" y2="150" stroke="#409eff" stroke-width="2" />

                        </svg>

                      </div>

                    </div>

                  </el-tab-pane>

                  <el-tab-pane label="详细数据表" name="table">

                    <div class="detail-table">

                      <el-table :data="detailData" style="width: 100%">

                        <el-table-column prop="observPoint" label="观测点" width="150" />

                        <el-table-column prop="resource" label="资源" width="150" />

                        <el-table-column prop="location" label="位置信息" width="150" />

                        <el-table-column prop="tunnel" label="隧道信息" width="150" />

                        <el-table-column prop="metrics" label="请求数量统计" />

                      </el-table>

                    </div>

                  </el-tab-pane>

                  <el-tab-pane label="柱状图" name="bar">

                    <div class="bar-chart">

                      <div class="mock-chart">

                        <div class="chart-bars">

                          <div v-for="i in 6" :key="i" class="chart-bar" :style="{ height: accessBarData[i-1] + '%' }"></div>

                        </div>

                        <div class="chart-x-axis">

                          <div v-for="i in 6" :key="i" class="x-axis-label">{{ accessBarLabels[i-1] }}</div>

                        </div>

                      </div>

                    </div>

                  </el-tab-pane>

                </el-tabs>

              </div>

            </div>

          </el-tab-pane>

          <el-tab-pane label="应用指标监控" name="appMetrics">

            <div class="app-metrics">

              <h3>应用指标监控</h3>

              <div class="metrics-controls">

                <el-form :inline="true" :model="metricsForm" class="demo-form-inline">

                  <el-form-item label="指标策略">

                    <el-select v-model="metricsForm.metric" placeholder="选择指标" style="width: 150px;">

                      <el-option label="网络流量监控" value="request_rate" />

                      <el-option label="分组聚合详情" value="response_time" />

                      <el-option label="服务端错误率" value="server_error" />

                    </el-select>

                  </el-form-item>

                  <el-form-item label="聚合函数">

                    <el-select v-model="metricsForm.aggregation" placeholder="选择聚合函数" style="width: 120px;">

                      <el-option label="平均值" value="avg" />

                      <el-option label="最大值" value="max" />

                      <el-option label="最小值" value="min" />

                      <el-option label="求和" value="sum" />

                    </el-select>

                  </el-form-item>

                  <el-form-item label="分组依据">

                    <el-select v-model="metricsForm.group" placeholder="选择分组字段" style="width: 120px;">

                      <el-option label="auto_service" value="auto_service" />

                      <el-option label="l7_protocol" value="l7_protocol" />

                    </el-select>

                  </el-form-item>

                  <el-form-item>

                    <el-checkbox v-model="metricsForm.tipSync">开启Tip同步</el-checkbox>

                  </el-form-item>

                </el-form>

              </div>

              <div class="metrics-chart">

                <div class="mock-chart">

                  <div class="chart-bars">

                    <div v-for="i in 60" :key="i" class="chart-bar" :style="{ height: metricsData[i-1] + '%' }"></div>

                  </div>

                  <div class="chart-x-axis">

                    <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

                  </div>

                </div>

              </div>

            </div>

          </el-tab-pane>

          <el-tab-pane label="端点列表" name="endpoints">

            <div class="endpoints">

              <h3>端点列表</h3>

              <el-table :data="endpointsData" style="width: 100%" @row-click="handleEndpointRowClick">

                <el-table-column prop="endpoint" label="端点" width="200" />

                <el-table-column prop="requestRate" label="网络流量监控" width="100" />

                <el-table-column prop="errorRate" label="服务端错误率" width="100" />

                <el-table-column prop="responseTime" label="分组聚合详情" width="100" />

                <el-table-column prop="qps" label="QPS" width="80" />

              </el-table>

            </div>

          </el-tab-pane>

          <el-tab-pane label="调用日志" name="callLog">

            <div class="call-log">

              <h3>调用日志</h3>

              <div class="log-chart">

                <div class="mock-chart">

                  <div class="chart-bars">

                    <div v-for="i in 60" :key="i" class="chart-bar" :style="{ height: logData[i-1] + '%' }"></div>

                  </div>

                  <div class="chart-x-axis">

                    <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

                  </div>

                </div>

              </div>

              <div class="log-table">

                <el-table :data="callLogData" style="width: 100%" @row-click="handleCallLogRowClick">

                  <el-table-column prop="startTime" label="开始时间" width="180" />

                  <el-table-column prop="client" label="客户端服务" width="150" />

                  <el-table-column prop="server" label="服务端列表" width="150" />

                  <el-table-column prop="application" label="应用名称" width="120" />

                  <el-table-column prop="requestType" label="请求类型" width="120" />

                  <el-table-column prop="requestDomain" label="处理域名" />

                </el-table>

              </div>

            </div>

          </el-tab-pane>

          <el-tab-pane label="调用链追踪" name="callTracing">

            <div class="call-tracing">

              <h3>调用链追踪</h3>

              <div class="tracing-topology">

                <div class="call-chain">

                  <div class="chain-node">

                    <div class="node-content">

                      <div class="node-label">web-shop</div>

                    </div>

                  </div>

                  <div class="chain-arrow">→</div>

                  <div class="chain-node">

                    <div class="node-content">

                      <div class="node-label">otel-agent</div>

                    </div>

                  </div>

                  <div class="chain-arrow">→</div>

                  <div class="chain-node">

                    <div class="node-content">

                      <div class="node-label">svc-user</div>

                    </div>

                  </div>

                  <div class="chain-arrow">→</div>

                  <div class="chain-node">

                    <div class="node-content">

                      <div class="node-label">database</div>

                    </div>

                  </div>

                </div>

              </div>

              <div class="tracing-table">

                <el-table :data="tracingData" style="width: 100%">

                  <el-table-column prop="spanId" label="Span ID" width="150" />

                  <el-table-column prop="parentSpanId" label="Parent Span ID" width="150" />

                  <el-table-column prop="service" label="服务" width="150" />

                  <el-table-column prop="operation" label="操作" width="150" />

                  <el-table-column prop="duration" label="平均耗时" width="100" />

                  <el-table-column prop="status" label="状态" width="80" />

                </el-table>

              </div>

            </div>

          </el-tab-pane>

          <el-tab-pane label="网络指标监控" name="netMetrics">

            <div class="net-metrics">

              <h3>网络指标监控</h3>

              <div class="metrics-controls">

                <el-form :inline="true" :model="netMetricsForm" class="demo-form-inline">

                  <el-form-item label="指标策略">

                    <el-select v-model="netMetricsForm.metric" placeholder="选择指标" style="width: 150px;">

                      <el-option label="流量" value="traffic" />

                      <el-option label="延迟" value="latency" />

                      <el-option label="丢包率" value="packet_loss" />

                    </el-select>

                  </el-form-item>

                  <el-form-item label="聚合函数">

                    <el-select v-model="netMetricsForm.aggregation" placeholder="选择聚合函数" style="width: 120px;">

                      <el-option label="平均值" value="avg" />

                      <el-option label="最大值" value="max" />

                      <el-option label="最小值" value="min" />

                      <el-option label="求和" value="sum" />

                    </el-select>

                  </el-form-item>

                  <el-form-item label="分组依据">

                    <el-select v-model="netMetricsForm.group" placeholder="选择分组字段" style="width: 120px;">

                      <el-option label="server_port" value="server_port" />

                      <el-option label="client_port" value="client_port" />

                    </el-select>

                  </el-form-item>

                  <el-form-item>

                    <el-checkbox v-model="netMetricsForm.tipSync">开启Tip同步</el-checkbox>

                  </el-form-item>

                </el-form>

              </div>

              <div class="metrics-chart">

                <div class="mock-chart">

                  <div class="chart-bars">

                    <div v-for="i in 60" :key="i" class="chart-bar" :style="{ height: netMetricsData[i-1] + '%' }"></div>

                  </div>

                  <div class="chart-x-axis">

                    <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

                  </div>

                </div>

              </div>

            </div>

          </el-tab-pane>

          <el-tab-pane label="流日志详情" name="flowLog">

            <div class="flow-log">

              <h3>流日志详情</h3>

              <div class="log-chart">

                <div class="mock-chart">

                  <div class="chart-bars">

                    <div v-for="i in 60" :key="i" class="chart-bar" :style="{ height: flowLogData[i-1] + '%' }"></div>

                  </div>

                  <div class="chart-x-axis">

                    <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

                  </div>

                </div>

              </div>

              <div class="log-table">

                <el-table :data="flowLogDataList" style="width: 100%" @row-click="handleFlowLogRowClick">

                  <el-table-column prop="startTime" label="开始时间" width="180" />

                  <el-table-column prop="client" label="客户端服务" width="150" />

                  <el-table-column prop="server" label="服务端列表" width="150" />

                  <el-table-column prop="application" label="应用名称" width="120" />

                  <el-table-column prop="requestType" label="请求类型" width="120" />

                  <el-table-column prop="requestDomain" label="处理域名" />

                </el-table>

              </div>

            </div>

          </el-tab-pane>

          <el-tab-pane label="NAT 前后流量" name="natTracing">

            <div class="nat-tracing">

              <h3>NAT 前后流量</h3>

              <el-table :data="natTracingData" style="width: 100%" @row-click="handleNatTracingRowClick">

                <el-table-column prop="client" label="客户端服务" width="150" />

                <el-table-column prop="server" label="服务端列表" width="150" />

                <el-table-column prop="requestRate" label="网络流量监控" width="100" />

                <el-table-column prop="errorRate" label="服务端错误率" width="100" />

                <el-table-column prop="responseTime" label="分组聚合详情" width="100" />

              </el-table>

            </div>

          </el-tab-pane>

          <el-tab-pane label="事件" name="events">

            <div class="events">

              <el-tabs v-model="eventsTab">

                <el-tab-pane label="资源变更事件" name="resource">

                  <div class="resource-events">

                    <h3>资源变更事件</h3>

                    <div class="log-chart">

                      <div class="mock-chart">

                        <div class="chart-bars">

                          <div v-for="i in 60" :key="i" class="chart-bar" :style="{ height: resourceEventsData[i-1] + '%' }"></div>

                        </div>

                        <div class="chart-x-axis">

                          <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

                        </div>

                      </div>

                    </div>

                    <div class="log-table">

                      <el-table :data="resourceEventsList" style="width: 100%">

                        <el-table-column prop="time" label="请求时间" width="180" />

                        <el-table-column prop="type" label="事件类型" width="120" />

                        <el-table-column prop="resource" label="资源" width="150" />

                        <el-table-column prop="message" label="事件信息" />

                      </el-table>

                    </div>

                  </div>

                </el-tab-pane>

                <el-tab-pane label="文件读写事件" name="file">

                  <div class="file-events">

                    <h3>文件读写事件</h3>

                    <div class="log-chart">

                      <div class="mock-chart">

                        <div class="chart-bars">

                          <div v-for="i in 60" :key="i" class="chart-bar" :style="{ height: fileEventsData[i-1] + '%' }"></div>

                        </div>

                        <div class="chart-x-axis">

                          <div v-for="i in 6" :key="i" class="x-axis-label">11:12</div>

                        </div>

                      </div>

                    </div>

                    <div class="log-table">

                      <el-table :data="fileEventsList" style="width: 100%">

                        <el-table-column prop="time" label="请求时间" width="180" />

                        <el-table-column prop="type" label="事件类型" width="80" />

                        <el-table-column prop="operation" label="操作" width="80" />

                        <el-table-column prop="process" label="进程" width="150" />

                        <el-table-column prop="filePath" label="文件路径" />

                      </el-table>

                    </div>

                  </div>

                </el-tab-pane>

              </el-tabs>

            </div>

          </el-tab-pane>

        </el-tabs>

      </div>

    </el-drawer>

  </div>

</template>



<script setup lang="ts">

// 生成模拟数据库（仅在组件挂载时调用一次，避免图表跳动）
const generateMockData = (max: number, min: number, count: number = 30) =>
  Array(count).fill(0).map(() => Math.random() * max + min)

import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'



// 抽屉功能演示

const drawerVisible = ref(false)



// 刷新标签签

const activeTab = ref('knowledge')



// 知识图谱标签

const knowledgeTab = ref('list')

const knowledgeSearch = ref('')

const knowledgeFilters = ref(['Client Universal Tag', 'Server Universal Tag'])



// 访问关系表单

const accessForm = ref({

  service: 'web-shop',

  roles: ['作为客户端服务', '作为服务端服务']

})

const accessTopologyTab = ref('topology')



// 应用指标表单

const metricsForm = ref({

  metric: 'request_rate',

  aggregation: 'avg',

  group: 'auto_service',

  tipSync: false

})



// 网络指标表单

const netMetricsForm = ref({

  metric: 'traffic',

  aggregation: 'sum',

  group: 'server_port',

  tipSync: false

})



// 事件标签

const eventsTab = ref('resource')



// 访问关系数据

const accessData = ref([

  {

    client: 'web-shop',

    server: '0.0.0.0',

    group: '进程/容器/服务',

    requestRate: '3.74',

    errorRate: '0%',

    responseTime: '2.33 ms'

  },

  {

    client: 'web-shop',

    server: 'otel-agent',

    group: '客户端/服务端',

    requestRate: '2.11',

    errorRate: '0%',

    responseTime: '354.41 us'

  },

  {

    client: 'web-shop',

    server: 'otel-agent',

    group: '进程/容器/服务',

    requestRate: '2.11',

    errorRate: '0%',

    responseTime: '369.16 us'

  },

  {

    client: 'web-shop',

    server: 'svc-user',

    group: '客户端/服务端',

    requestRate: '1.87',

    errorRate: '0%',

    responseTime: '2.21 ms'

  },

  {

    client: 'web-shop',

    server: 'svc-user',

    group: '进程/容器/服务',

    requestRate: '1.87',

    errorRate: '0%',

    responseTime: '2.22 ms'

  }

])



// 详细数据表数据

const detailData = ref([

  {

    observPoint: '进程/容器',

    resource: 'web-shop',

    location: 'c7i-00b1',

    tunnel: 'N/A',

    metrics: '网络流量监控: 3.74, 分组聚合详情: 2.33 ms'

  },

  {

    observPoint: '进程/容器/服务',

    resource: 'eth0',

    location: 'c7i-00b1',

    tunnel: 'N/A',

    metrics: '网络流量监控: 2.11, 分组聚合详情: 369.16 us'

  },

  {

    observPoint: '客户端/服务端',

    resource: 'web-shop-container',

    location: 'c7i-00b1',

    tunnel: 'N/A',

    metrics: '网络流量监控: 2.11, 分组聚合详情: 354.41 us'

  },

  {

    observPoint: '服务端进程',

    resource: 'otel-agent-container',

    location: 'c7i-00b2',

    tunnel: 'N/A',

    metrics: '网络流量监控: 2.11, 分组聚合详情: 350.20 us'

  },

  {

    observPoint: '进程/容器',

    resource: 'eth0',

    location: 'c7i-00b2',

    tunnel: 'N/A',

    metrics: '网络流量监控: 2.11, 分组聚合详情: 345.10 us'

  },

  {

    observPoint: '进程/容器',

    resource: 'otel-agent',

    location: 'c7i-00b2',

    tunnel: 'N/A',

    metrics: '网络流量监控: 2.11, 分组聚合详情: 340.00 us'

  }

])



// 访问关系柱状图数据

const accessBarData = ref([30, 40, 50, 60, 70, 80])

const accessBarLabels = ref(['进程/容器', '进程/容器/服务', '客户端/服务端', '服务端进程', '进程/容器', '默认网络'])



// 应用指标监控模块

const metricsData = ref([])



// 网络指标监控模块

const netMetricsData = ref([])



// 端点列表数据

const endpointsData = ref([

  {

    endpoint: 'web-shop',

    requestRate: '3.74',

    errorRate: '0%',

    responseTime: '2.33 ms',

    qps: '1000'

  },

  {

    endpoint: 'otel-agent',

    requestRate: '2.11',

    errorRate: '0%',

    responseTime: '354.41 us',

    qps: '800'

  },

  {

    endpoint: 'svc-user',

    requestRate: '1.87',

    errorRate: '0%',

    responseTime: '2.21 ms',

    qps: '600'

  }

])



// 调用日志数据

const logData = ref([])

const callLogData = ref([

  {

    startTime: '2023-09-21 15:08:36.947975',

    client: 'web-shop',

    server: 'otel-agent',

    application: 'HTTP',

    requestType: 'POST',

    requestDomain: 'otel-agent.open-telemetry:11888'

  },

  {

    startTime: '2023-09-21 15:08:35.523535',

    client: 'web-shop',

    server: 'otel-agent',

    application: 'HTTP',

    requestType: 'POST',

    requestDomain: 'otel-agent.open-telemetry:11888'

  }

])



// 调用数据流

const tracingData = ref([

  {

    spanId: 'span-1',

    parentSpanId: '',

    service: 'web-shop',

    operation: 'POST /api',

    duration: '10ms',

    status: '成功'

  },

  {

    spanId: 'span-2',

    parentSpanId: 'span-1',

    service: 'otel-agent',

    operation: 'process request',

    duration: '5ms',

    status: '成功'

  },

  {

    spanId: 'span-3',

    parentSpanId: 'span-2',

    service: 'svc-user',

    operation: 'get user info',

    duration: '3ms',

    status: '成功'

  }

])



// 流日志详情相关数据库

const flowLogData = ref([])

const flowLogDataList = ref([

  {

    startTime: '2023-09-21 15:08:36.947975',

    client: 'web-shop',

    server: 'otel-agent',

    application: 'HTTP',

    requestType: 'POST',

    requestDomain: 'otel-agent.open-telemetry:11888'

  },

  {

    startTime: '2023-09-21 15:08:35.523535',

    client: 'web-shop',

    server: 'otel-agent',

    application: 'HTTP',

    requestType: 'POST',

    requestDomain: 'otel-agent.open-telemetry:11888'

  }

])



// NAT 追踪数据

const natTracingData = ref([

  {

    client: '192.168.1.100:12345',

    server: '10.0.0.1:8080',

    requestRate: '1.2K',

    errorRate: '0%',

    responseTime: '10ms'

  },

  {

    client: '192.168.1.101:23456',

    server: '10.0.0.2:8080',

    requestRate: '800',

    errorRate: '0%',

    responseTime: '8ms'

  }

])



// 资源变更事件数据流

const resourceEventsData = ref([])

const resourceEventsList = ref([

  {

    time: '2023-09-21 15:00:00',

    type: '资源创建',

    resource: 'web-shop pod',

    message: '资源创建事件: web-shop pod'

  },

  {

    time: '2023-09-21 14:30:00',

    type: '配置变更',

    resource: 'otel-agent deployment',

    message: '部署状态otel-agent deployment 是否正常'

  }

])



// 文件读写事件数据库

const fileEventsData = ref([])

onMounted(() => {
  metricsData.value = generateMockData(80, 20, 60)
  netMetricsData.value = generateMockData(80, 20, 60)
  logData.value = generateMockData(80, 20, 60)
  flowLogData.value = generateMockData(80, 20, 60)
  resourceEventsData.value = generateMockData(80, 20, 60)
  fileEventsData.value = generateMockData(80, 20, 60)
})


const fileEventsList = ref([

  {

    time: '2023-09-21 15:08:36.947975',

    type: 'IO',

    operation: '读',

    process: 'web-shop',

    filePath: '/app/config.yaml'

  },

  {

    time: '2023-09-21 15:08:35.523535',

    type: 'IO',

    operation: '写',

    process: 'web-shop',

    filePath: '/app/logs/app.log'

  }

])



// 打开抽屉
const openDrawer = () => {

  drawerVisible.value = true

}



// 复制标签
const copyTag = (key: string, value: string) => {
  navigator.clipboard.writeText(`${key}: ${value}`).then(() => {
    ElMessage.success('复制成功')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}



// 处理访问关系行点击

const handleAccessRowClick = (row: any) => {

  }



// 处理端点列表行点击

const handleEndpointRowClick = (row: any) => {

  }



// 处理调用日志行点击
const handleCallLogRowClick = (row: any) => {

  }



// 处理流日志行点击

const handleFlowLogRowClick = (row: any) => {

  }



// 处理 NAT 追踪行点击

const handleNatTracingRowClick = (row: any) => {

  }

</script>



<style scoped>

.app-drawer-content {

  padding: 20px;

}



.app-drawer-content h2 {

  margin-top: 0;

  margin-bottom: 20px;

  font-size: 18px;

  font-weight: bold;

  color: #303133;

}



.app-drawer-content p {

  margin-bottom: 10px;

  color: #606266;

}



.app-drawer-content ul {

  margin-bottom: 20px;

  padding-left: 20px;

  color: #606266;

}



.app-drawer-content li {

  margin-bottom: 5px;

}



.demo-button {

  margin-top: 20px;

}



.drawer-demo {

  padding: 20px;

}



.knowledge-graph {

  padding: 20px 0;

}



.knowledge-search {

  margin-bottom: 20px;

}



.knowledge-filters {

  margin-bottom: 20px;

}



.knowledge-content {

  margin-top: 20px;

}



.knowledge-content h4 {

  margin-top: 20px;

  margin-bottom: 10px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.tag-list {

  margin-bottom: 20px;

}



.tag-item {

  display: flex;

  align-items: center;

  margin-bottom: 10px;

  padding: 10px;

  background-color: #f5f7fa;

  border-radius: 4px;

}



.tag-key {

  font-weight: bold;

  margin-right: 10px;

  color: #303133;

}



.tag-value {

  flex: 1;

  color: #606266;

}



.mock-graph {

  position: relative;

  width: 100%;

  height: 400px;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  padding: 20px;

}



.graph-node {

  position: absolute;

  cursor: pointer;

  transition: all 0.3s ease;

}



.graph-node:hover {

  transform: scale(1.05);

}



.center-node {

  top: 50px;

  left: 400px;

}



.left-node {

  top: 250px;

  left: 200px;

}



.right-node {

  top: 250px;

  left: 600px;

}



.bottom-node {

  top: 350px;

  left: 400px;

}



.node-content {

  padding: 10px;

  background-color: white;

  border: 1px solid #409eff;

  border-radius: 4px;

  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);

  text-align: center;

}



.node-label {

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.graph-connections {

  position: absolute;

  top: 0;

  left: 0;

  z-index: 0;

  pointer-events: none;

}



.access-relation {

  padding: 20px 0;

}



.access-controls {

  margin-bottom: 20px;

}



.access-table {

  margin-bottom: 30px;

}



.access-table h4 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.access-topology {

  margin-top: 30px;

}



.access-topology h4 {

  margin-top: 0;

  margin-bottom: 15px;

  font-size: 14px;

  font-weight: bold;

  color: #303133;

}



.topology-content {

  padding: 20px;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  margin-bottom: 20px;

}



.mock-topology {

  position: relative;

  width: 100%;

  height: 200px;

}



.topology-node {

  position: absolute;

  top: 50%;

  transform: translateY(-50%);

  cursor: pointer;

  transition: all 0.3s ease;

}



.topology-node:hover {

  transform: translateY(-50%) scale(1.05);

}



.topology-node:nth-child(1) {

  left: 100px;

}



.topology-node:nth-child(2) {

  left: 200px;

}



.topology-node:nth-child(3) {

  left: 300px;

}



.topology-node:nth-child(4) {

  left: 400px;

}



.topology-node:nth-child(5) {

  left: 500px;

}



.topology-node:nth-child(6) {

  left: 600px;

}



.node-icon {

  font-size: 24px;

  margin-bottom: 5px;

}



.topology-connections {

  position: absolute;

  top: 0;

  left: 0;

  z-index: 0;

  pointer-events: none;

}



.detail-table {

  margin-top: 20px;

}



.bar-chart {

  padding: 20px;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

}



.app-metrics,

.net-metrics {

  padding: 20px 0;

}



.metrics-controls {

  margin-bottom: 20px;

}



.metrics-chart {

  padding: 20px;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  margin-top: 20px;

}



.endpoints,

.call-log,

.call-tracing,

.flow-log,

.nat-tracing,

.events {

  padding: 20px 0;

}



.log-chart {

  padding: 20px;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  margin-bottom: 20px;

}



.log-table {

  margin-top: 20px;

}



.tracing-topology {

  margin-bottom: 30px;

}



.call-chain {

  display: flex;

  align-items: center;

  gap: 20px;

  padding: 20px;

  background-color: #f5f7fa;

  border-radius: 4px;

  margin-bottom: 20px;

}



.chain-node {

  flex: 1;

  text-align: center;

}



.chain-arrow {

  font-size: 20px;

  color: #409eff;

  font-weight: bold;

}



.resource-events,

.file-events {

  padding: 20px 0;

}



/* 模拟图表样式 */

.mock-chart {

  position: relative;

  width: 100%;

  height: 200px;

  overflow: hidden;

}



.chart-bars {

  display: flex;

  align-items: flex-end;

  height: 80%;

  gap: 2px;

  padding: 0 10px;

}



.chart-bar {

  flex: 1;

  min-height: 2px;

  background-color: #409eff;

  border-radius: 2px 2px 0 0;

  transition: height 0.3s ease;

}



.chart-x-axis {

  display: flex;

  justify-content: space-between;

  height: 20%;

  padding: 0 10px;

  margin-top: 5px;

}



.x-axis-label {

  font-size: 10px;

  color: #909399;

  text-align: center;

  flex: 1;

}



@media (max-width: 768px) {

  .tag-item {

    flex-direction: column;

    align-items: flex-start;

  }

  

  .tag-key {

    margin-bottom: 5px;

  }

  

  .call-chain {

    flex-direction: column;

    gap: 10px;

  }

  

  .chain-arrow {

    transform: rotate(90deg);

  }

  

  .topology-node {

    position: static;

    margin-bottom: 20px;

    transform: none;

  }

  

  .topology-node:hover {

    transform: scale(1.05);

  }

  

  .topology-connections {

    display: none;

  }

}

</style>