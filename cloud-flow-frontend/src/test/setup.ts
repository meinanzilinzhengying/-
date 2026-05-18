import { vi } from 'vitest'

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: vi.fn((key: string) => store[key] || null),
    setItem: vi.fn((key: string, value: string) => { store[key] = value }),
    removeItem: vi.fn((key: string) => { delete store[key] }),
    clear: vi.fn(() => { store = {} }),
  }
})()
Object.defineProperty(window, 'localStorage', { value: localStorageMock })

// Mock fetch
global.fetch = vi.fn()

// Mock crypto.subtle for AES-GCM tests
const mockCryptoKey = {} as CryptoKey
Object.defineProperty(global, 'crypto', {
  value: {
    subtle: {
      digest: vi.fn(() => Promise.resolve(new ArrayBuffer(32))),
      importKey: vi.fn(() => Promise.resolve(mockCryptoKey)),
      encrypt: vi.fn(() => Promise.resolve(new ArrayBuffer(64))),
      decrypt: vi.fn(() => Promise.resolve(new TextEncoder().encode('test-token'))),
    },
    getRandomValues: vi.fn((arr: Uint8Array) => {
      for (let i = 0; i < arr.length; i++) arr[i] = Math.floor(Math.random() * 256)
      return arr
    }),
  },
})

// Mock navigator
Object.defineProperty(window, 'navigator', {
  value: { userAgent: 'test-agent' },
})

// Mock screen
Object.defineProperty(window, 'screen', {
  value: { width: 1920, height: 1080 },
})

// Mock Intl.DateTimeFormat - need to handle both with and without 'new'
const OriginalDateTimeFormat = Intl.DateTimeFormat

function MockDateTimeFormat(
  this: Intl.DateTimeFormat,
  locales?: string | string[],
  options?: Intl.DateTimeFormatOptions
) {
  // Handle both new and non-new calls
  if (!(this instanceof MockDateTimeFormat)) {
    return new (MockDateTimeFormat as any)(locales, options)
  }
  return new OriginalDateTimeFormat(locales, options)
}

MockDateTimeFormat.prototype.resolvedOptions = function() {
  return { timeZone: 'Asia/Shanghai' } as Intl.ResolvedDateTimeFormatOptions
}

// Copy static properties
Object.setPrototypeOf(MockDateTimeFormat, OriginalDateTimeFormat)
Object.setPrototypeOf(MockDateTimeFormat.prototype, OriginalDateTimeFormat.prototype)

// @ts-ignore
Intl.DateTimeFormat = MockDateTimeFormat as typeof Intl.DateTimeFormat
