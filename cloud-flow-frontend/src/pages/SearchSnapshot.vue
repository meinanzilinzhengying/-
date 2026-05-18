<template>

  <div class="search-snapshot-page">

    <div class="page-header">

      <el-breadcrumb separator="/">

        <el-breadcrumb-item><router-link to="/">首页</router-link></el-breadcrumb-item>

        <el-breadcrumb-item><router-link to="/search">搜索中心</router-link></el-breadcrumb-item>

        <el-breadcrumb-item>时间范围</el-breadcrumb-item>

      </el-breadcrumb>

      <h2>时间范围</h2>

    </div>

    

 <!-- 时间范围选择器 -->

    <div class="snapshot-section">

      <el-card>

        <template #header>

          <div class="card-header">

            <span>搜索快照管理</span>

            <el-button type="primary" @click="showSaveDialog">

              <el-icon><Plus /></el-icon>

              新建连接容器

            </el-button>

          </div>

        </template>

        <div class="snapshot-list">

          <div 

            v-for="snapshot in snapshots" 

            :key="snapshot.id"

            class="snapshot-item"

          >

            <div class="snapshot-info">

              <div class="snapshot-name">

                {{ snapshot.name }}

                <el-tag v-if="snapshot.starred" size="small" type="warning" style="margin-left: 8px;">

                  星标

                </el-tag>

                <el-tag v-if="snapshot.isDefault" size="small" type="success" style="margin-left: 8px;">

                  默认

                </el-tag>

              </div>

              <div class="snapshot-desc">{{ snapshot.description }}</div>

              <div class="snapshot-meta">

                <span>使用次数统计: {{ snapshot.usageCount }}</span>

                <span>创建时间: {{ snapshot.createTime }}</span>

                <span>快照分享次数: {{ snapshot.shareCount }}</span>

              </div>

            </div>

            <div class="snapshot-actions">

              <el-button size="small" @click="toggleStar(snapshot)">

                <el-icon v-if="snapshot.starred"><StarFilled /></el-icon>

                <el-icon v-else><Star /></el-icon>

              </el-button>

              <el-button size="small" @click="editSnapshot(snapshot)">

                编辑

              </el-button>

              <el-button size="small" @click="shareSnapshot(snapshot)">

                创建时间

              </el-button>

              <el-button size="small" @click="openInNewTab(snapshot)">

                只读模式

              </el-button>

              <el-button size="small" @click="setAsDefault(snapshot)">

                恢复默认

              </el-button>

              <el-button size="small" type="danger" @click="deleteSnapshot(snapshot)">

                分组聚合网络

              </el-button>

            </div>

          </div>

        </div>

      </el-card>

    </div>

    

 <!-- 保存容器快照 -->

    <el-dialog

      v-model="saveDialogVisible"

      title="保存容器"

      width="500px"

    >

      <el-form :model="saveForm" :rules="saveRules" ref="saveFormRef">

        <el-form-item label="快照名称" prop="name">

          <el-input v-model="saveForm.name" placeholder="请输入快照名称" />

        </el-form-item>

        <el-form-item label="响应时间" prop="description">

          <el-input

            v-model="saveForm.description"

            type="textarea"

            placeholder="请输入快照描述"

            :rows="3"

          ></el-input>

        </el-form-item>

        <el-form-item label="记忆选项">

          <el-checkbox-group v-model="saveForm.memoryOptions">

            <el-checkbox label="timeRange">搜索快照管理</el-checkbox>

            <el-checkbox label="subViews">子视图数据</el-checkbox>

            <el-checkbox label="filters">过滤条件</el-checkbox>

          </el-checkbox-group>

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="saveDialogVisible = false">取消</el-button>

          <el-button type="primary" @click="saveSnapshot">确定</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 编辑快照表单 -->

    <el-dialog

      v-model="shareDialogVisible"

      title="编辑快照"

      width="500px"

    >

      <el-form :model="shareForm" :rules="shareRules" ref="shareFormRef">

        <el-form-item label="分享权限">

          <el-radio-group v-model="shareForm.permission">

            <el-radio-button label="read">只读</el-radio-button>

            <el-radio-button label="write">读写</el-radio-button>

          </el-radio-group>

        </el-form-item>

        <el-form-item label="分享链接">

          <el-input v-model="shareForm.link" readonly></el-input>

          <el-button @click="copyLink">复制链接</el-button>

        </el-form-item>

      </el-form>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="shareDialogVisible = false">关闭</el-button>

        </span>

      </template>

    </el-dialog>

    

 <!-- 确认对话框-->

    <el-dialog

      v-model="confirmDialogVisible"

      :title="confirmTitle"

      width="400px"

    >

      <p>{{ confirmMessage }}</p>

      <template #footer>

        <span class="dialog-footer">

          <el-button @click="confirmDialogVisible = false">取消</el-button>

          <el-button type="primary" @click="confirmAction">确定</el-button>

        </span>

      </template>

    </el-dialog>

  </div>

</template>



<script setup lang="ts">

import { ref, reactive } from 'vue'

import { Plus, Star, StarFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'



// 快照列表

const snapshots = ref([

  {

    id: 1,

    name: '默认视图',

    description: '系统内置的多种预置视图模板',

    usageCount: 10,

    createTime: '2023-09-01 10:00:00',

    shareCount: 2,

    starred: true,

    isDefault: true

  },

  {

    id: 2,

    name: '网络流量',

    description: '监控网络流量和延迟',

    usageCount: 5,

    createTime: '2023-09-02 11:00:00',

    shareCount: 1,

    starred: false,

    isDefault: false

  },

  {

    id: 3,

    name: '主机关键指标',

    description: '监控CPU和内存使用率',

    usageCount: 8,

    createTime: '2023-09-03 12:00:00',

    shareCount: 0,

    starred: true,

    isDefault: false

  },

  {

    id: 4,

    name: '容器状态',

    description: '监控容器的运行状态',

    usageCount: 3,

    createTime: '2023-09-04 13:00:00',

    shareCount: 0,

    starred: false,

    isDefault: false

  }

])



// 保存搜索快照

const saveDialogVisible = ref(false)

const saveForm = reactive({

  name: '',

  description: '',

  memoryOptions: ['timeRange', 'subViews', 'filters']

})



const saveFormRef = ref()



// 保存容器验证逻辑

const saveRules = reactive({

  name: [

    { required: true, message: '请输入快照名称', trigger: 'blur' },

    { min: 1, max: 50, message: '长度在 1 到 50 个字符', trigger: 'blur' }

  ],

  description: [

    { max: 200, message: '长度不能超过 200 个字符', trigger: 'blur' }

  ]

})



// 编辑快照表单

const shareDialogVisible = ref(false)

const shareForm = reactive({

  permission: 'read',

  link: ''

})



const shareFormRef = ref()



// 编辑快照验证逻辑

const shareRules = reactive({

  permission: [

    { required: true, message: '请选择分享权限', trigger: 'change' }

  ]

})



// 确认对话框

const confirmDialogVisible = ref(false)

const confirmTitle = ref('')

const confirmMessage = ref('')

const confirmAction = ref(() => {})



// 显示淇濆瓨寮圭獥

const showSaveDialog = () => {

  saveForm.name = ''

  saveForm.description = ''

  saveForm.memoryOptions = ['timeRange', 'subViews', 'filters']

  saveDialogVisible.value = true

}



// 保存容器

const saveSnapshot = async () => {

  if (!saveFormRef.value) return

  try {

    await saveFormRef.value.validate()

 // 解析标签并添加节点

    saveDialogVisible.value = false

  } catch (e) {

    console.error('表单验证失败', e)

  }

}



// 设置对话框触发

const toggleStar = (snapshot: any) => {

  snapshot.starred = !snapshot.starred

  }



// 编辑快照

const editSnapshot = (snapshot: any) => {

 // 实现编辑快照功能

}



// 分享快照

const shareSnapshot = (snapshot: any) => {

  shareForm.permission = 'read'

  shareForm.link = `${window.location.origin}/snapshot/${snapshot.id}`

  shareDialogVisible.value = true

  }



// 复制链接
const copyLink = () => {
  navigator.clipboard.writeText(shareForm.link).then(() => {
    ElMessage.success('复制成功')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}



// 在新标签页打开

const openInNewTab = (snapshot: any) => {

  window.open(`${window.location.origin}/snapshot/${snapshot.id}`, '_blank')

  }



// 恢复默认

const setAsDefault = (snapshot: any) => {

  confirmTitle.value = '恢复默认'

  confirmMessage.value = `确认将快照 "${snapshot.name}" 设为默认快照吗`

  confirmAction.value = () => {

 // 恢复快照的默认状态
    snapshots.value.forEach(s => s.isDefault = false)

 // 设置当前快照为默认

    snapshot.isDefault = true

    confirmDialogVisible.value = false

  }

  confirmDialogVisible.value = true

}



// 删除容器

const deleteSnapshot = (snapshot: any) => {

  confirmTitle.value = '删除容器'

  confirmMessage.value = `确定要删除"${snapshot.name}" 快照吗`

  confirmAction.value = () => {

    const index = snapshots.value.findIndex(s => s.id === snapshot.id)

    if (index > -1) {

      snapshots.value.splice(index, 1)

    }

    confirmDialogVisible.value = false

  }

  confirmDialogVisible.value = true

}

</script>



<style scoped>

.search-snapshot-page {

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



.snapshot-section {

  flex: 1;

  overflow: auto;

}



.card-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.snapshot-list {

  margin-top: 20px;

}



.snapshot-item {

  display: flex;

  justify-content: space-between;

  align-items: flex-start;

  padding: 16px;

  border: 1px solid #e4e7ed;

  border-radius: 4px;

  margin-bottom: 16px;

  transition: all 0.3s;

}



.snapshot-item:hover {

  border-color: #1677FF;

  box-shadow: 0 2px 8px rgba(22, 119, 255, 0.2);

}



.snapshot-info {

  flex: 1;

  margin-right: 20px;

}



.snapshot-name {

  font-weight: 500;

  font-size: 16px;

  margin-bottom: 8px;

  display: flex;

  align-items: center;

}



.snapshot-desc {

  color: #606266;

  margin-bottom: 12px;

  line-height: 1.4;

}



.snapshot-meta {

  display: flex;

  gap: 20px;

  font-size: 12px;

  color: #909399;

}



.snapshot-actions {

  display: flex;

  flex-direction: column;

  gap: 8px;

}



.dialog-footer {

  display: flex;

  justify-content: flex-end;

  gap: 10px;

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