/**
 * 图片懒加载指令 v-lazy
 * 使用 IntersectionObserver API 实现高性能懒加载
 * 支持占位图、加载失败处理、淡入动画
 */

import type { Directive, DirectiveBinding } from 'vue'

interface LazyOptions {
  placeholder?: string
  error?: string
  rootMargin?: string
  threshold?: number
}

const defaultOptions: LazyOptions = {
  placeholder: 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7',
  error: '',
  rootMargin: '50px 0px',
  threshold: 0.01
}

// 创建 IntersectionObserver 实例（单例模式）
let observer: IntersectionObserver | null = null

function getObserver(options: LazyOptions): IntersectionObserver {
  if (!observer) {
    observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const img = entry.target as HTMLImageElement
            loadImage(img)
            observer?.unobserve(img)
          }
        })
      },
      {
        rootMargin: options.rootMargin,
        threshold: options.threshold
      }
    )
  }
  return observer
}

function loadImage(img: HTMLImageElement) {
  const src = img.dataset.src
  if (!src) return

  // 添加加载中样式
  img.classList.add('lazy-loading')

  const tempImg = new Image()

  tempImg.onload = () => {
    img.src = src
    img.classList.remove('lazy-loading')
    img.classList.add('lazy-loaded')
    delete img.dataset.src
  }

  tempImg.onerror = () => {
    img.classList.remove('lazy-loading')
    img.classList.add('lazy-error')
    // 使用错误占位图
    const errorSrc = img.dataset.error || defaultOptions.error
    if (errorSrc) {
      img.src = errorSrc
    }
    delete img.dataset.src
  }

  tempImg.src = src
}

const lazyLoad: Directive<HTMLImageElement, string> = {
  mounted(el, binding: DirectiveBinding<string>) {
    const options: LazyOptions = {
      ...defaultOptions,
      ...(typeof binding.value === 'object' ? binding.value : {})
    }

    // 保存原始 src
    const src = typeof binding.value === 'string' ? binding.value : binding.value?.src
    if (!src) return

    el.dataset.src = src

    // 设置占位图
    if (!el.src || el.src === window.location.href) {
      el.src = options.placeholder!
    }

    // 保存错误处理图
    if (options.error) {
      el.dataset.error = options.error
    }

    // 添加基础样式
    el.style.opacity = '0'
    el.style.transition = 'opacity 0.3s ease-in-out'

    // 开始观察
    getObserver(options).observe(el)
  },

  updated(el, binding: DirectiveBinding<string>) {
    const newSrc = typeof binding.value === 'string' ? binding.value : binding.value?.src
    const oldSrc = el.dataset.src

    if (newSrc && newSrc !== oldSrc) {
      el.dataset.src = newSrc
      el.classList.remove('lazy-loaded')
      getObserver(defaultOptions).observe(el)
    }
  },

  unmounted(el) {
    observer?.unobserve(el)
  }
}

export default lazyLoad
