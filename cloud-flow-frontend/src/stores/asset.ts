import { defineStore } from 'pinia';
import { api } from '../utils/api';
import type {
  ChangeEvent, ResourcePool, CloudPlatform, Region, AvailabilityZone,
  Server, Host, VPC, Subnet, Router, DhcpServer, IpAddress, AssetSearchParams
} from '../types';
import type { PaginatedResponse } from './index';

interface AssetState {
  changeEvents: ChangeEvent[];
  resourcePools: ResourcePool[];
  cloudPlatforms: CloudPlatform[];
  regions: Region[];
  availabilityZones: AvailabilityZone[];
  servers: Server[];
  hosts: Host[];
  vpcs: VPC[];
  subnets: Subnet[];
  routers: Router[];
  dhcpServers: DhcpServer[];
  ipAddresses: IpAddress[];
  loadingCount: number;
  error: string | null;
  // 分页信息
  total: Record<string, number>;
}

export const useAssetStore = defineStore('asset', {
  state: (): AssetState => ({
    changeEvents: [],
    resourcePools: [],
    cloudPlatforms: [],
    regions: [],
    availabilityZones: [],
    servers: [],
    hosts: [],
    vpcs: [],
    subnets: [],
    routers: [],
    dhcpServers: [],
    ipAddresses: [],
    loadingCount: 0,
    error: null,
    total: {}
  }),

  getters: {
    getChangeEvents: (state) => state.changeEvents,
    getResourcePools: (state) => state.resourcePools,
    getCloudPlatforms: (state) => state.cloudPlatforms,
    getRegions: (state) => state.regions,
    getAvailabilityZones: (state) => state.availabilityZones,
    getServers: (state) => state.servers,
    getHosts: (state) => state.hosts,
    getVpcs: (state) => state.vpcs,
    getSubnets: (state) => state.subnets,
    getRouters: (state) => state.routers,
    getDhcpServers: (state) => state.dhcpServers,
    getIpAddresses: (state) => state.ipAddresses,
    getLoading: (state) => state.loadingCount > 0,
    getError: (state) => state.error
  },

  actions: {
    async fetchChangeEvents(params?: AssetSearchParams): Promise<ChangeEvent[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.asset.getChangeEvents(params);
        // 处理分页响应：如果返回包含 data 和 total 字段，则提取 data
        const paginated = response as unknown as PaginatedResponse<ChangeEvent>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.changeEvents = paginated.data;
          this.total['changeEvents'] = paginated.total;
          return paginated.data;
        }
        const events = response as unknown as ChangeEvent[];
        this.changeEvents = events;
        return events;
      } catch (e) {
        this.error = (e as Error).message || '获取变更事件失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchResourcePools(): Promise<ResourcePool[]> {
      this.loadingCount++;

      try {
        const pools = await api.asset.getResourcePools();
        this.resourcePools = pools;
        return pools;
      } catch (e) {
        this.error = (e as Error).message || '获取资源池失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchCloudPlatforms(): Promise<CloudPlatform[]> {
      this.loadingCount++;

      try {
        const platforms = await api.asset.getCloudPlatforms();
        this.cloudPlatforms = platforms;
        return platforms;
      } catch (e) {
        this.error = (e as Error).message || '获取云平台失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchRegions(cloudPlatformId?: string): Promise<Region[]> {
      this.loadingCount++;

      try {
        const regions = await api.asset.getRegions(cloudPlatformId);
        this.regions = regions;
        return regions;
      } catch (e) {
        this.error = (e as Error).message || '获取区域失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchAvailabilityZones(regionId?: string): Promise<AvailabilityZone[]> {
      this.loadingCount++;

      try {
        const zones = await api.asset.getAvailabilityZones(regionId);
        this.availabilityZones = zones;
        return zones;
      } catch (e) {
        this.error = (e as Error).message || '获取可用区失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchServers(params?: AssetSearchParams): Promise<Server[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.asset.getServers(params);
        const paginated = response as unknown as PaginatedResponse<Server>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.servers = paginated.data;
          this.total['servers'] = paginated.total;
          return paginated.data;
        }
        const servers = response as unknown as Server[];
        this.servers = servers;
        return servers;
      } catch (e) {
        this.error = (e as Error).message || '获取云服务器失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchHosts(params?: AssetSearchParams): Promise<Host[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.asset.getHosts(params);
        const paginated = response as unknown as PaginatedResponse<Host>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.hosts = paginated.data;
          this.total['hosts'] = paginated.total;
          return paginated.data;
        }
        const hosts = response as unknown as Host[];
        this.hosts = hosts;
        return hosts;
      } catch (e) {
        this.error = (e as Error).message || '获取宿主机失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchVpcs(params?: AssetSearchParams): Promise<VPC[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.asset.getVpcs(params);
        const paginated = response as unknown as PaginatedResponse<VPC>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.vpcs = paginated.data;
          this.total['vpcs'] = paginated.total;
          return paginated.data;
        }
        const vpcs = response as unknown as VPC[];
        this.vpcs = vpcs;
        return vpcs;
      } catch (e) {
        this.error = (e as Error).message || '获取VPC失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchSubnets(params?: AssetSearchParams): Promise<Subnet[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.asset.getSubnets(params);
        const paginated = response as unknown as PaginatedResponse<Subnet>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.subnets = paginated.data;
          this.total['subnets'] = paginated.total;
          return paginated.data;
        }
        const subnets = response as unknown as Subnet[];
        this.subnets = subnets;
        return subnets;
      } catch (e) {
        this.error = (e as Error).message || '获取子网失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchRouters(params?: AssetSearchParams): Promise<Router[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.asset.getRouters(params);
        const paginated = response as unknown as PaginatedResponse<Router>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.routers = paginated.data;
          this.total['routers'] = paginated.total;
          return paginated.data;
        }
        const routers = response as unknown as Router[];
        this.routers = routers;
        return routers;
      } catch (e) {
        this.error = (e as Error).message || '获取路由器失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchDhcpServers(params?: AssetSearchParams): Promise<DhcpServer[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.asset.getDhcpServers(params);
        const paginated = response as unknown as PaginatedResponse<DhcpServer>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.dhcpServers = paginated.data;
          this.total['dhcpServers'] = paginated.total;
          return paginated.data;
        }
        const dhcpServers = response as unknown as DhcpServer[];
        this.dhcpServers = dhcpServers;
        return dhcpServers;
      } catch (e) {
        this.error = (e as Error).message || '获取DHCP服务器失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchIpAddresses(params?: AssetSearchParams): Promise<IpAddress[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.asset.getIpAddresses(params);
        const paginated = response as unknown as PaginatedResponse<IpAddress>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.ipAddresses = paginated.data;
          this.total['ipAddresses'] = paginated.total;
          return paginated.data;
        }
        const ipAddresses = response as unknown as IpAddress[];
        this.ipAddresses = ipAddresses;
        return ipAddresses;
      } catch (e) {
        this.error = (e as Error).message || '获取IP地址失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    }
  }
});