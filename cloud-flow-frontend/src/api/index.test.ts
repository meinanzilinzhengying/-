import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { getToken, setToken, removeToken, apiRequest, unwrapPaginatedResponse, resetRedirect } from './index'

// Mock dependencies
vi.mock('element-plus', () => ({ ElMessage: { error: vi.fn() } }))
vi.mock('../router', () => ({ default: { push: vi.fn() }, clearAuthCache: vi.fn() }))

describe('Token Management', () => {
  beforeEach(() => {
    localStorage.clear()
    resetRedirect()
    vi.clearAllMocks()
  })

  describe('setToken and getToken', () => {
    it('should encrypt and store token', async () => {
      await setToken('my-test-token')
      expect(localStorage.setItem).toHaveBeenCalled()
      
      const token = await getToken()
      // With our mock, decrypt returns 'test-token'
      expect(token).toBeTruthy()
    })

    it('should return null when no token stored', async () => {
      const token = await getToken()
      expect(token).toBeNull()
    })
  })

  describe('removeToken', () => {
    it('should remove token from localStorage', () => {
      removeToken()
      expect(localStorage.removeItem).toHaveBeenCalledWith('cf_token')
    })
  })
})

describe('unwrapPaginatedResponse', () => {
  it('should extract items and total from paginated response', () => {
    const response = { data: [{ id: 1 }, { id: 2 }], total: 100 }
    const result = unwrapPaginatedResponse(response)
    expect(result.items).toHaveLength(2)
    expect(result.total).toBe(100)
  })

  it('should handle non-paginated array response', () => {
    const response = [{ id: 1 }, { id: 2 }]
    const result = unwrapPaginatedResponse(response)
    expect(result.items).toHaveLength(2)
    expect(result.total).toBe(0)
  })

  it('should handle null/undefined response', () => {
    const result = unwrapPaginatedResponse(null)
    expect(result.items).toHaveLength(0)
    expect(result.total).toBe(0)
  })
})

describe('apiRequest', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(global.fetch as any).mockReset()
  })

  it('should make successful GET request', async () => {
    const mockData = { success: true, data: { id: 1 } }
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockData),
    })

    const result = await apiRequest('/test')
    expect(result.success).toBe(true)
  })

  it('should include Authorization header when token exists', async () => {
    await setToken('test-token')
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ success: true, data: {} }),
    })

    await apiRequest('/test')
    expect(global.fetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: expect.stringContaining('Bearer'),
        }),
      })
    )
  })

  it('should throw on HTTP error', async () => {
    ;(global.fetch as any).mockResolvedValueOnce({
      ok: false,
      status: 500,
    })

    await expect(apiRequest('/test')).rejects.toThrow()
  })
})
