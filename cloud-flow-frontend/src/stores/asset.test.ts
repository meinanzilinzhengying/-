import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAssetStore } from './asset'

// Mock API
vi.mock('../utils/api', () => ({
  api: {
    asset: {
      getChangeEvents: vi.fn(),
      getResourcePools: vi.fn(),
      getCloudPlatforms: vi.fn(),
      getRegions: vi.fn(),
      getAvailabilityZones: vi.fn(),
      getServers: vi.fn(),
      getHosts: vi.fn(),
      getVpcs: vi.fn(),
      getSubnets: vi.fn(),
      getRouters: vi.fn(),
      getDhcpServers: vi.fn(),
      getIpAddresses: vi.fn(),
    }
  }
}))

import { api } from '../utils/api'

describe('Asset Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('should initialize with empty state', () => {
    const store = useAssetStore()
    expect(store.servers).toEqual([])
    expect(store.hosts).toEqual([])
    expect(store.vpcs).toEqual([])
    expect(store.subnets).toEqual([])
    expect(store.routers).toEqual([])
    expect(store.dhcpServers).toEqual([])
    expect(store.ipAddresses).toEqual([])
    expect(store.loadingCount).toBe(0)
    expect(store.error).toBeNull()
  })

  describe('getters', () => {
    it('should return correct loading state', () => {
      const store = useAssetStore()
      expect(store.getLoading).toBe(false)
      
      store.loadingCount = 1
      expect(store.getLoading).toBe(true)
    })

    it('should return error state', () => {
      const store = useAssetStore()
      expect(store.getError).toBeNull()
      
      store.error = 'Test error'
      expect(store.getError).toBe('Test error')
    })
  })

  describe('fetchServers', () => {
    it('should fetch and store servers', async () => {
      const mockServers = [
        { id: '1', name: 'server-1' },
        { id: '2', name: 'server-2' }
      ]
      ;(api.asset.getServers as any).mockResolvedValue(mockServers)

      const store = useAssetStore()
      const result = await store.fetchServers()

      expect(result).toHaveLength(2)
      expect(store.servers).toHaveLength(2)
      expect(store.loadingCount).toBe(0)
    })

    it('should handle paginated response', async () => {
      const mockResponse = {
        data: [{ id: '1', name: 'server-1' }],
        total: 100
      }
      ;(api.asset.getServers as any).mockResolvedValue(mockResponse)

      const store = useAssetStore()
      const result = await store.fetchServers()

      expect(result).toHaveLength(1)
      expect(store.total['servers']).toBe(100)
    })

    it('should handle errors', async () => {
      const mockError = new Error('Network error')
      ;(api.asset.getServers as any).mockRejectedValue(mockError)

      const store = useAssetStore()
      
      await expect(store.fetchServers()).rejects.toThrow()
      expect(store.error).toBe('Network error')
    })
  })

  describe('fetchHosts', () => {
    it('should fetch and store hosts', async () => {
      const mockHosts = [
        { id: '1', name: 'host-1' },
        { id: '2', name: 'host-2' }
      ]
      ;(api.asset.getHosts as any).mockResolvedValue(mockHosts)

      const store = useAssetStore()
      const result = await store.fetchHosts()

      expect(result).toHaveLength(2)
      expect(store.hosts).toHaveLength(2)
      expect(store.loadingCount).toBe(0)
    })
  })

  describe('fetchVpcs', () => {
    it('should fetch and store vpcs', async () => {
      const mockVpcs = [
        { id: '1', name: 'vpc-1' }
      ]
      ;(api.asset.getVpcs as any).mockResolvedValue(mockVpcs)

      const store = useAssetStore()
      const result = await store.fetchVpcs()

      expect(result).toHaveLength(1)
      expect(store.vpcs).toHaveLength(1)
    })
  })

  describe('fetchSubnets', () => {
    it('should fetch and store subnets', async () => {
      const mockSubnets = [
        { id: '1', name: 'subnet-1' }
      ]
      ;(api.asset.getSubnets as any).mockResolvedValue(mockSubnets)

      const store = useAssetStore()
      const result = await store.fetchSubnets()

      expect(result).toHaveLength(1)
      expect(store.subnets).toHaveLength(1)
    })
  })

  describe('fetchRouters', () => {
    it('should fetch and store routers', async () => {
      const mockRouters = [
        { id: '1', name: 'router-1' }
      ]
      ;(api.asset.getRouters as any).mockResolvedValue(mockRouters)

      const store = useAssetStore()
      const result = await store.fetchRouters()

      expect(result).toHaveLength(1)
      expect(store.routers).toHaveLength(1)
    })
  })

  describe('fetchDhcpServers', () => {
    it('should fetch and store dhcp servers', async () => {
      const mockDhcpServers = [
        { id: '1', name: 'dhcp-1' }
      ]
      ;(api.asset.getDhcpServers as any).mockResolvedValue(mockDhcpServers)

      const store = useAssetStore()
      const result = await store.fetchDhcpServers()

      expect(result).toHaveLength(1)
      expect(store.dhcpServers).toHaveLength(1)
    })
  })

  describe('fetchIpAddresses', () => {
    it('should fetch and store ip addresses', async () => {
      const mockIpAddresses = [
        { id: '1', address: '192.168.1.1' }
      ]
      ;(api.asset.getIpAddresses as any).mockResolvedValue(mockIpAddresses)

      const store = useAssetStore()
      const result = await store.fetchIpAddresses()

      expect(result).toHaveLength(1)
      expect(store.ipAddresses).toHaveLength(1)
    })
  })

  describe('fetchChangeEvents', () => {
    it('should fetch and store change events', async () => {
      const mockEvents = [
        { id: '1', type: 'create' }
      ]
      ;(api.asset.getChangeEvents as any).mockResolvedValue(mockEvents)

      const store = useAssetStore()
      const result = await store.fetchChangeEvents()

      expect(result).toHaveLength(1)
      expect(store.changeEvents).toHaveLength(1)
    })
  })

  describe('fetchResourcePools', () => {
    it('should fetch and store resource pools', async () => {
      const mockPools = [
        { id: '1', name: 'pool-1' }
      ]
      ;(api.asset.getResourcePools as any).mockResolvedValue(mockPools)

      const store = useAssetStore()
      const result = await store.fetchResourcePools()

      expect(result).toHaveLength(1)
      expect(store.resourcePools).toHaveLength(1)
    })
  })

  describe('fetchCloudPlatforms', () => {
    it('should fetch and store cloud platforms', async () => {
      const mockPlatforms = [
        { id: '1', name: 'platform-1' }
      ]
      ;(api.asset.getCloudPlatforms as any).mockResolvedValue(mockPlatforms)

      const store = useAssetStore()
      const result = await store.fetchCloudPlatforms()

      expect(result).toHaveLength(1)
      expect(store.cloudPlatforms).toHaveLength(1)
    })
  })

  describe('fetchRegions', () => {
    it('should fetch and store regions', async () => {
      const mockRegions = [
        { id: '1', name: 'region-1' }
      ]
      ;(api.asset.getRegions as any).mockResolvedValue(mockRegions)

      const store = useAssetStore()
      const result = await store.fetchRegions()

      expect(result).toHaveLength(1)
      expect(store.regions).toHaveLength(1)
    })
  })

  describe('fetchAvailabilityZones', () => {
    it('should fetch and store availability zones', async () => {
      const mockZones = [
        { id: '1', name: 'zone-1' }
      ]
      ;(api.asset.getAvailabilityZones as any).mockResolvedValue(mockZones)

      const store = useAssetStore()
      const result = await store.fetchAvailabilityZones()

      expect(result).toHaveLength(1)
      expect(store.availabilityZones).toHaveLength(1)
    })
  })
})
