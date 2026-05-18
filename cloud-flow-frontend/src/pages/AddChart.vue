<template>

  <div class="add-chart">

    <div class="page-header">

      <el-breadcrumb separator="/">

        <el-breadcrumb-item><router-link to="/views/list">视频列表</router-link></el-breadcrumb-item>

        <el-breadcrumb-item><router-link to="/views/detail">检查网络流量监控</router-link></el-breadcrumb-item>

        <el-breadcrumb-item>{{ isEdit ? '编辑图表' : '添加图表' }}</el-breadcrumb-item>

      </el-breadcrumb>

      <h2>{{ isEdit ? '编辑图表' : '添加图表' }}</h2>

    </div>

    

    <div class="form-section">

      <el-form :model="chartForm" :rules="chartRules" ref="chartFormRef">

        <el-card>

          <template #header>

            <div class="card-header">

              <span>基础配置</span>

            </div>

          </template>

          <el-form-item label="图表策略" prop="name">

            <el-input v-model="chartForm.name" placeholder="请输入图表名称" />

          </el-form-item>

          <el-form-item label="图表类型" prop="type">

            <el-select v-model="chartForm.type" placeholder="请选择图表类型">

              <el-option label="折线图" value="line" />

              <el-option label="柱状图" value="bar" />

              <el-option label="饼图" value="pie" />

              <el-option label="仪表盘" value="gauge" />

              <el-option label="散点图" value="scatter" />

            </el-select>

          </el-form-item>

          <el-form-item label="响应时间" prop="description">

            <el-input

              v-model="chartForm.description"

              type="textarea"

              placeholder="请输入图表描述"

              :rows="3"

            ></el-input>

          </el-form-item>

        </el-card>

        

        <el-card class="mt-4">

          <template #header>

            <div class="card-header">

              <span>数据源配置</span>

            </div>

          </template>

          <el-form-item label="数据库来源" prop="dataSource">

            <el-select v-model="chartForm.dataSource" placeholder="请选择数据源">

              <el-option label="Prometheus" value="prometheus" />

              <el-option label="Grafana" value="grafana" />

              <el-option label="DeepFlow" value="deepflow" />

            </el-select>

          </el-form-item>

          <el-form-item label="搜索快照管理" prop="timeRange">

            <el-select v-model="chartForm.timeRange" placeholder="请选择时间范围">

              <el-option label="最近5分钟" value="5m" />

              <el-option label="最近15分钟" value="15m" />

              <el-option label="最近30分钟" value="30m" />

              <el-option label="最近1小时" value="1h" />

              <el-option label="最近6小时" value="6h" />

              <el-option label="最近12小时" value="12h" />

              <el-option label="最近24小时" value="24h" />

            </el-select>

          </el-form-item>

        </el-card>

        

        <el-card class="mt-4">

          <template #header>

            <div class="card-header">

              <span>指标配置</span>

            </div>

          </template>

          <el-form-item label="指标" prop="metric">

            <el-select v-model="chartForm.metric" placeholder="请选择指标">

              <el-option label="CPU使用率" value="cpu_usage" />

              <el-option label="内存使用率" value="mem_usage" />

              <el-option label="网络流量" value="network_traffic" />

              <el-option label="服务分组聚合详情" value="response_time" />

              <el-option label="错误率" value="error_rate" />

              <el-option label="QPS" value="qps" />

            </el-select>

          </el-form-item>

          <el-form-item label="聚合类型" prop="aggregation">

            <el-select v-model="chartForm.aggregation" placeholder="请选择聚合函数">

              <el-option label="平均值" value="avg" />

              <el-option label="最大值" value="max" />

              <el-option label="最小值" value="min" />

              <el-option label="求和" value="sum" />

              <el-option label="计数" value="count" />

            </el-select>

          </el-form-item>

          <el-form-item label="分组依据" prop="groupBy">

            <el-select v-model="chartForm.groupBy" placeholder="请选择分组字段" multiple>

              <el-option label="服务" value="service" />

              <el-option label="服务分组" value="service_group" />

              <el-option label="区域" value="region" />

              <el-option label="主机名" value="host" />

              <el-option label="容器" value="container" />

            </el-select>

          </el-form-item>

        </el-card>

        

        <el-card class="mt-4">

          <template #header>

            <div class="card-header">

              <span>样式配置</span>

            </div>

          </template>

          <el-form-item label="图表颜色" prop="color">

            <el-color-picker v-model="chartForm.color" show-alpha></el-color-picker>

          </el-form-item>

          <el-form-item label="显示图例" prop="showLegend">

            <el-switch v-model="chartForm.showLegend"></el-switch>

          </el-form-item>

          <el-form-item label="显示网格" prop="showGrid">

            <el-switch v-model="chartForm.showGrid"></el-switch>

          </el-form-item>

        </el-card>

        

        <div class="form-actions">

          <el-button @click="cancel">取消</el-button>

          <el-button type="primary" @click="submitForm">确定</el-button>

        </div>

      </el-form>

    </div>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive, onMounted } from 'vue'

import { useRouter, useRoute } from 'vue-router'



const router = useRouter()

const route = useRoute()



// 是否为编辑模式

const isEdit = ref(false)



// 检查采集器状态功能

const chartForm = reactive({

  name: '',

  type: '',

  description: '',

  dataSource: '',

  timeRange: '1h',

  metric: '',

  aggregation: 'avg',

  groupBy: [],

  color: '#1677FF',

  showLegend: true,

  showGrid: true

})



const chartFormRef = ref()



// 表单验证规则

const chartRules = reactive({

  name: [

    { required: true, message: '请输入图表名称', trigger: 'blur' },

    { min: 1, max: 50, message: '长度在 1 到 50 个字符', trigger: 'blur' }

  ],

  type: [

    { required: true, message: '请选择图表类型', trigger: 'change' }

  ],

  dataSource: [

    { required: true, message: '请选择数据源', trigger: 'change' }

  ],

  metric: [

    { required: true, message: '请选择指标', trigger: 'change' }

  ],

  aggregation: [

    { required: true, message: '请选择聚合函数', trigger: 'change' }

  ]

})



// 创建图表表单

onMounted(() => {

 // 从路由参数中获取图表ID

  const id = route.query.id

  if (id) {

    isEdit.value = true

 // 模拟获取图表数据流

 // 插入图表创建数据

    chartForm.name = 'CPU使用率'

    chartForm.type = 'line'

    chartForm.description = 'CPU使用率图表模块'

    chartForm.dataSource = 'deepflow'

    chartForm.timeRange = '1h'

    chartForm.metric = 'cpu_usage'

    chartForm.aggregation = 'avg'

    chartForm.groupBy = ['service', 'host']

    chartForm.color = '#1677FF'

    chartForm.showLegend = true

    chartForm.showGrid = true

  }

})



// 取消

const cancel = () => {

  router.push('/views/detail')

}



// 重置搜索条件

const submitForm = async () => {

  if (!chartFormRef.value) return

  try {

    await chartFormRef.value.validate()

 // 实现提交保存功能

    router.push('/views/detail')

  } catch (e) {

    console.error('表单验证失败', e)

  }

}

</script>



<style scoped>

.add-chart {

  background-color: white;

  border-radius: 4px;

  padding: 24px;

  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);

  height: 100%;

  display: flex;

  flex-direction: column;

  gap: 24px;

}



.page-header h2 {

  margin: 8px 0 0 0;

  font-size: 18px;

  font-weight: bold;

  color: #303133;

}



.form-section {

  flex: 1;

  overflow: auto;

}



.card-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.mt-4 {

  margin-top: 24px;

}



.form-actions {

  display: flex;

  justify-content: flex-end;

  gap: 10px;

  padding-top: 24px;

  border-top: 1px solid #e4e7ed;

  margin-top: 24px;

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