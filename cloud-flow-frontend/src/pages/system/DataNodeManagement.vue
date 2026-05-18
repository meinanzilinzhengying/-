<template>
  <div class="data-node-management">
 <!-- 操作按钮 -->
    <div class="action-buttons mb-4">
      <el-button type="primary" @click="handleCreateDataTable">新建数据库表</el-button>
    </div>

 <!-- 搜索 -->
    <div class="search-filter mb-4">
      <el-form :inline="true" :model="dataNodeSearchForm" class="demo-form-inline">
        <el-form-item label="数据库表列表">
          <el-input v-model="dataNodeSearchForm.keyword" placeholder="查找" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleDataNodeSearch">查找</el-button>
        </el-form-item>
      </el-form>
    </div>

 <!-- 数据库节点列表 -->
    <el-table :data="dataTables" style="width: 100%">
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column prop="tableCollection" label="数据库表集合" min-width="200" />
      <el-table-column prop="creationTime" label="创建时间" width="180" />
      <el-table-column prop="timeGranularity" label="时间粒度" width="100" />
      <el-table-column prop="retentionTime" label="保存时长" width="100" />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="scope">
          <el-tag :type="scope.row.status === '正常' ? 'success' : 'danger'">{{ scope.row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="scope">
          <el-button size="small" @click="editDataTable(scope.row)">编辑</el-button>
          <el-button size="small" type="danger" v-if="!scope.row.isSystem" @click="deleteDataTable(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

 <!-- 分页 -->
    <div class="pagination mt-4">
      <el-pagination
        background
        layout="prev, pager, next, jumper"
        :total="dataNodeTotal"
        :page-size="pageSize"
        :current-page="dataNodeCurrentPage"
        @current-change="handleDataNodeCurrentChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const dataNodeSearchForm = ref({
  keyword: ''
})

const dataTables = ref([
  {
    name: 'network_traffic',
    tableCollection: 'network',
    creationTime: '2026-04-01 10:00:00',
    timeGranularity: '1min',
    retentionTime: '30天',
    status: '正常',
    isSystem: false
  }
])

const dataNodeTotal = ref(100)
const dataNodeCurrentPage = ref(1)
const pageSize = ref(20)

const handleCreateDataTable = () => {
}

const handleDataNodeSearch = () => {
}

const handleDataNodeCurrentChange = (page: number) => {
  dataNodeCurrentPage.value = page
}

const editDataTable = (row: any) => {
}

const deleteDataTable = (row: any) => {
}
</script>

<style scoped>
.data-node-management {
  padding: 20px;
}
</style>