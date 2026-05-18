/**
 * Web Vitals 性能监控工具
 * 收集 Core Web Vitals 指标并上报
 */

export interface WebVitalsMetric {
  name: string
  value: number
  rating: 'good' | 'needs-improvement' | 'poor'
  delta?: number
  id: string
  navigationType?: string
}

// 性能指标阈值（根据 Google 标准）
const thresholds: Record<string, { good: number; poor: number }> = {
  LCP: { good: 2500, poor: 4000 },        // Largest Contentful Paint
  FID: { good: 100, poor: 300 },          // First Input Delay
  CLS: { good: 0.1, poor: 0.25 },         // Cumulative Layout Shift
  FCP: { good: 1800, poor: 3000 },        // First Contentful Paint
  TTFB: { good: 800, poor: 1800 },        // Time to First Byte
  INP: { good: 200, poor: 500 },          // Interaction to Next Paint
}

/**
 * 获取性能评分
 */
function getRating(name: string, value: number): WebVitalsMetric['rating'] {
  const threshold = thresholds[name]
  if (!threshold) return 'good'
  
  if (value <= threshold.good) return 'good'
  if (value <= threshold.poor) return 'needs-improvement'
  return 'poor'
}

/**
 * 生成唯一 ID
 */
function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
}

/**
 * 上报性能指标
 */
function reportMetric(metric: WebVitalsMetric) {
  // 控制台输出（开发环境）
  if (import.meta.env.DEV) {
    const color = metric.rating === 'good' ? '#52c41a' : 
                  metric.rating === 'needs-improvement' ? '#faad14' : '#f5222d'
    console.log(
      `%c[Web Vitals] ${metric.name}: ${metric.value.toFixed(2)} (${metric.rating})`,
      `color: ${color}; font-weight: bold;`
    )
  }

  // 发送到分析服务（生产环境）
  if (import.meta.env.PROD && navigator.sendBeacon) {
    navigator.sendBeacon('/api/metrics/web-vitals', JSON.stringify(metric))
  }

  // 触发自定义事件，供其他组件使用
  window.dispatchEvent(new CustomEvent('web-vitals', { detail: metric }))
}

/**
 * 监听 LCP (Largest Contentful Paint)
 */
export function observeLCP() {
  if (!('PerformanceObserver' in window)) return

  const observer = new PerformanceObserver((list) => {
    const entries = list.getEntries()
    const lastEntry = entries[entries.length - 1] as PerformanceEntry & { startTime: number }
    
    const metric: WebVitalsMetric = {
      name: 'LCP',
      value: lastEntry.startTime,
      rating: getRating('LCP', lastEntry.startTime),
      id: generateId()
    }
    
    reportMetric(metric)
  })

  observer.observe({ entryTypes: ['largest-contentful-paint'] })
  
  // 页面卸载前断开观察
  window.addEventListener('pagehide', () => observer.disconnect())
}

/**
 * 监听 FID (First Input Delay)
 */
export function observeFID() {
  if (!('PerformanceObserver' in window)) return

  const observer = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      const fidEntry = entry as PerformanceEntry & { processingStart: number; startTime: number }
      const value = fidEntry.processingStart - fidEntry.startTime
      
      const metric: WebVitalsMetric = {
        name: 'FID',
        value,
        rating: getRating('FID', value),
        id: generateId()
      }
      
      reportMetric(metric)
    }
  })

  observer.observe({ entryTypes: ['first-input'] })
}

/**
 * 监听 CLS (Cumulative Layout Shift)
 */
export function observeCLS() {
  if (!('PerformanceObserver' in window)) return

  let clsValue = 0
  let sessionEntries: PerformanceEntry[] = []

  const observer = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      const layoutShift = entry as PerformanceEntry & { value: number; hadRecentInput: boolean }
      
      // 只计算没有用户输入的布局偏移
      if (!layoutShift.hadRecentInput) {
        clsValue += layoutShift.value
        sessionEntries.push(entry)
      }
    }
  })

  observer.observe({ entryTypes: ['layout-shift'] })

  // 页面卸载前报告最终值
  window.addEventListener('pagehide', () => {
    const metric: WebVitalsMetric = {
      name: 'CLS',
      value: clsValue,
      rating: getRating('CLS', clsValue),
      id: generateId()
    }
    reportMetric(metric)
    observer.disconnect()
  })
}

/**
 * 监听 FCP (First Contentful Paint)
 */
export function observeFCP() {
  if (!('PerformanceObserver' in window)) return

  const observer = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      const paintEntry = entry as PerformanceEntry & { startTime: number }
      
      if (paintEntry.name === 'first-contentful-paint') {
        const metric: WebVitalsMetric = {
          name: 'FCP',
          value: paintEntry.startTime,
          rating: getRating('FCP', paintEntry.startTime),
          id: generateId()
        }
        
        reportMetric(metric)
        observer.disconnect()
      }
    }
  })

  observer.observe({ entryTypes: ['paint'] })
}

/**
 * 监听 TTFB (Time to First Byte)
 */
export function observeTTFB() {
  if (!('performance' in window)) return

  const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming
  if (!navigation) return

  const value = navigation.responseStart - navigation.startTime
  
  const metric: WebVitalsMetric = {
    name: 'TTFB',
    value,
    rating: getRating('TTFB', value),
    id: generateId()
  }
  
  reportMetric(metric)
}

/**
 * 监听 INP (Interaction to Next Paint)
 */
export function observeINP() {
  if (!('PerformanceObserver' in window)) return

  let inpValue = 0

  const observer = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      const eventEntry = entry as PerformanceEntry & { duration: number; interactionId: number }
      
      // 只计算有 interactionId 的事件
      if (eventEntry.interactionId > 0) {
        inpValue = Math.max(inpValue, eventEntry.duration)
      }
    }
  })

  observer.observe({ entryTypes: ['event'] })

  // 页面卸载前报告
  window.addEventListener('pagehide', () => {
    if (inpValue > 0) {
      const metric: WebVitalsMetric = {
        name: 'INP',
        value: inpValue,
        rating: getRating('INP', inpValue),
        id: generateId()
      }
      reportMetric(metric)
    }
    observer.disconnect()
  })
}

/**
 * 初始化所有 Web Vitals 监控
 */
export function initWebVitals() {
  // 延迟到页面加载完成后初始化，避免影响性能
  if (document.readyState === 'complete') {
    startObserving()
  } else {
    window.addEventListener('load', startObserving)
  }
}

function startObserving() {
  // 使用 requestIdleCallback 在浏览器空闲时初始化
  if ('requestIdleCallback' in window) {
    requestIdleCallback(() => {
      observeLCP()
      observeFID()
      observeCLS()
      observeFCP()
      observeTTFB()
      observeINP()
    })
  } else {
    setTimeout(() => {
      observeLCP()
      observeFID()
      observeCLS()
      observeFCP()
      observeTTFB()
      observeINP()
    }, 1000)
  }
}

/**
 * 获取当前性能数据摘要
 */
export function getPerformanceSummary(): Record<string, number | undefined> {
  const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming
  
  if (!navigation) return {}

  return {
    dns: navigation.domainLookupEnd - navigation.domainLookupStart,
    tcp: navigation.connectEnd - navigation.connectStart,
    ttfb: navigation.responseStart - navigation.startTime,
    download: navigation.responseEnd - navigation.responseStart,
    domParse: navigation.domInteractive - navigation.responseEnd,
    domReady: navigation.domContentLoadedEventEnd - navigation.startTime,
    loadComplete: navigation.loadEventEnd - navigation.startTime,
  }
}

export default {
  init: initWebVitals,
  observeLCP,
  observeFID,
  observeCLS,
  observeFCP,
  observeTTFB,
  observeINP,
  getPerformanceSummary
}
