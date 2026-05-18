<script setup lang="ts">
import { ref, computed } from 'vue'
import { ArrowDown, Download, Star, StarFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { Snapshot, SearchResult, SearchHistory } from '../types'

// 快照列表
const snapshots = ref<Snapshot[]>([
  {
    id: 1,
    name: '服务监控',
    description: '监控所有服务的响应时间和错误率',
    usageCount: 10,
    createTime: '2023-09-01 10:00:00',
    starred: true,
    isDefault: true
  },
  {
    id: 2,
    name: '网络流量',
    description: '监控网络流量和延迟',
    usageCount: 5,
    createTime: '2023-09-02 11:00:00',
    starred: false,
    isDefault: false
  },
  {
    id: 3,
    name: '主机性能',
    description: '监控主机的CPU和内存使用率',
    usageCount: 8,
    createTime: '2023-09-03 12:00:00',
    starred: true,
    isDefault: false
  },
  {
    id: 4,
    name: '数据库性能',
    description: '监控数据库查询性能和连接数',
    usageCount: 3,
    createTime: '2023-09-04 13:00:00',
    starred: false,
    isDefault: false
  },
  {
    id: 5,
    name: '应用错误',
    description: '追踪应用错误和异常',
    usageCount: 12,
    createTime: '2023-09-05 14:00:00',
    starred: false,
    isDefault: false
  }
])

// 搜索历史
const searchHistory = ref<SearchHistory[]>([
  { id: 1, query: 'error AND level:>Warning', time: '2023-09-01 10:00:00' },
  { id: 2, query: 'service:payment AND latency:>1s', time: '2023-09-02 11:00:00' },
  { id: 3, query: 'cpu:>80%', time: '2023-09-03 12:00:00' }
])

// 保存快照弹窗
const saveDialogVisible = ref(false)
const newSnapshotName = ref('')
const newSnapshotDescription = ref('')

// 搜索框
const searchInput = ref('')
const snapshotSearch = ref('')
const searchResults = ref<SearchResult[]>([])
const isSearching = ref(false)

// 搜索中的加载状态
const isLoading = ref(false)

// 搜索建议
const showSuggestions = ref(false)
const suggestions = ref<string[]>([])

// 分享弹窗
const shareDialogVisible = ref(false)
const shareLink = ref('')
const shareExpiry = ref('24h')

// 确认弹窗
const confirmDialogVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmAction = ref<() => void>(() => {})

// 快照详情
const showDetail = ref(false)
const selectedSnapshot = ref<Snapshot | null>(null)
const detailStyle = ref<Record<string, string>>({})

// 过滤后的快照
const filteredSnapshots = computed(() => {
  let filtered = snapshots.value.filter(snapshot => !snapshot.starred)
  if (snapshotSearch.value) {
    const search = snapshotSearch.value.toLowerCase()
    filtered = filtered.filter(snapshot =>
      snapshot.name.toLowerCase().includes(search) ||
      snapshot.description.toLowerCase().includes(search)
    )
  }
  return filtered
})

// 标星快照
const starredSnapshots = computed(() => {
  let starred = snapshots.value.filter(snapshot => snapshot.starred)
  if (snapshotSearch.value) {
    const search = snapshotSearch.value.toLowerCase()
    starred = starred.filter(snapshot =>
      snapshot.name.toLowerCase().includes(search) ||
      snapshot.description.toLowerCase().includes(search)
    )
  }
  return starred
})

// 搜索
const handleSearch = () => {
  if (!searchInput.value.trim()) return
  isSearching.value = true
  setTimeout(() => {
    searchResults.value = [
      { id: 1, time: '2023-09-01 10:00:00', message: 'Error: connection timeout', level: 'error' },
      { id: 2, time: '2023-09-01 10:01:00', message: 'Warning: high latency', level: 'warning' },
      { id: 3, time: '2023-09-01 10:02:00', message: 'Info: request completed', level: 'info' }
    ]
    isSearching.value = false
  }, 500)
}

// 保存快照
const saveSnapshot = () => {
  if (!newSnapshotName.value.trim()) return

  const newSnapshot: Snapshot = {
    id: Date.now(),
    name: newSnapshotName.value,
    description: newSnapshotDescription.value,
    usageCount: 0,
    createTime: new Date().toISOString().replace('T', ' ').substring(0, 19),
    starred: false,
    isDefault: false
  }

  snapshots.value.push(newSnapshot)
  saveDialogVisible.value = false
  newSnapshotName.value = ''
  newSnapshotDescription.value = ''
}

// 加载快照
const loadSnapshot = (snapshot: Snapshot) => {
  searchInput.value = 'snapshot query'
  showDetail.value = false
  const index = snapshots.value.findIndex(s => s.id === snapshot.id)
  if (index !== -1) {
    snapshots.value[index].usageCount++
  }
}

// 删除快照
const deleteSnapshot = (snapshot: Snapshot) => {
  confirmDialogVisible.value = true
  confirmTitle.value = '删除快照'
  confirmMessage.value = `确定要删除快照"${snapshot.name}"吗？`
  confirmAction.value = () => {
    const index = snapshots.value.findIndex(s => s.id === snapshot.id)
    if (index !== -1) {
      snapshots.value.splice(index, 1)
    }
    confirmDialogVisible.value = false
  }
}

// 标星快照
const toggleStar = (snapshot: Snapshot) => {
  const index = snapshots.value.findIndex(s => s.id === snapshot.id)
  if (index !== -1) {
    snapshots.value[index].starred = !snapshots.value[index].starred
  }
}

// 查看快照详情
const viewSnapshotDetail = (snapshot: Snapshot) => {
  selectedSnapshot.value = snapshot
  showDetail.value = true
  detailStyle.value = {
    backgroundColor: '#f5f7fa',
    padding: '20px',
    borderRadius: '4px'
  }
}

// 复制搜索查询
const copySearchQuery = (query: string) => {
  navigator.clipboard.writeText(query).then(() => {
    ElMessage.success('复制成功')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
  searchInput.value = query
  showSuggestions.value = false
}

// 清除搜索历史
const clearSearchHistory = () => {
  searchHistory.value = []
}

// 分享快照
const shareSnapshot = (snapshot: Snapshot) => {
  shareDialogVisible.value = true
  shareLink.value = `${window.location.origin}/snapshot/${snapshot.id}`
}

// 删除搜索历史
const deleteSearchHistory = (id: number) => {
  const index = searchHistory.value.findIndex(h => h.id === id)
  if (index !== -1) {
    searchHistory.value.splice(index, 1)
  }
}
</script>