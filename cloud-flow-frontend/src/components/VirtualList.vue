<template>
  <div
    ref="containerRef"
    class="virtual-list-container"
    :style="{ height: `${containerHeight}px` }"
    @scroll="handleScroll"
  >
    <!-- 占位元素，用于撑开滚动条 -->
    <div
      class="virtual-list-phantom"
      :style="{ height: `${totalHeight}px` }"
    />
    
    <!-- 可视区域内容 -->
    <div
      class="virtual-list-content"
      :style="{ transform: `translateY(${offset}px)` }"
    >
      <div
        v-for="item in visibleItems"
        :key="item.key"
        class="virtual-list-item"
        :style="{ height: `${itemHeight}px` }"
      >
        <slot :item="item.data" :index="item.index" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts" generic="T">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'

interface Props {
  items: T[]
  itemHeight: number
  containerHeight: number
  buffer?: number
  keyField?: keyof T
}

const props = withDefaults(defineProps<Props>(), {
  buffer: 5,
  keyField: 'id' as keyof T
})

const containerRef = ref<HTMLElement>()
const scrollTop = ref(0)

// 计算总高度
const totalHeight = computed(() => {
  return props.items.length * props.itemHeight
})

// 计算可视区域的起始索引
const startIndex = computed(() => {
  const index = Math.floor(scrollTop.value / props.itemHeight)
  return Math.max(0, index - props.buffer)
})

// 计算可视区域的结束索引
const endIndex = computed(() => {
  const visibleCount = Math.ceil(props.containerHeight / props.itemHeight)
  const index = startIndex.value + visibleCount + props.buffer * 2
  return Math.min(props.items.length, index)
})

// 计算偏移量
const offset = computed(() => {
  return startIndex.value * props.itemHeight
})

// 可视区域的数据项
const visibleItems = computed(() => {
  const result = []
  for (let i = startIndex.value; i < endIndex.value; i++) {
    const item = props.items[i]
    if (item) {
      result.push({
        data: item,
        index: i,
        key: String(item[props.keyField] ?? i)
      })
    }
  }
  return result
})

// 使用 requestAnimationFrame 优化滚动性能
let rafId: number | null = null

const handleScroll = () => {
  if (rafId !== null) return
  
  rafId = requestAnimationFrame(() => {
    if (containerRef.value) {
      scrollTop.value = containerRef.value.scrollTop
    }
    rafId = null
  })
}

// 滚动到指定索引
const scrollToIndex = (index: number) => {
  if (containerRef.value) {
    containerRef.value.scrollTop = index * props.itemHeight
  }
}

// 滚动到顶部
const scrollToTop = () => {
  scrollToIndex(0)
}

// 滚动到底部
const scrollToBottom = () => {
  scrollToIndex(props.items.length - 1)
}

// 暴露方法给父组件
defineExpose({
  scrollToIndex,
  scrollToTop,
  scrollToBottom
})

// 监听数据变化，重置滚动位置
watch(() => props.items.length, () => {
  scrollTop.value = 0
  if (containerRef.value) {
    containerRef.value.scrollTop = 0
  }
})

onUnmounted(() => {
  if (rafId !== null) {
    cancelAnimationFrame(rafId)
  }
})
</script>

<style scoped>
.virtual-list-container {
  position: relative;
  overflow-y: auto;
  overflow-x: hidden;
  -webkit-overflow-scrolling: touch;
}

.virtual-list-phantom {
  position: absolute;
  left: 0;
  top: 0;
  right: 0;
  z-index: -1;
}

.virtual-list-content {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
}

.virtual-list-item {
  box-sizing: border-box;
}
</style>
