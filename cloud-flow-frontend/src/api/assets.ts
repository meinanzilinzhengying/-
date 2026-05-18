import { apiRequest, type ApiResponse } from './index';

// 解包 ApiResponse，返回 data 字段
function unwrap<T>(promise: Promise<ApiResponse<T>>): Promise<T> {
  return promise.then(r => r.data);
}

export const assetsApi = {
  getChangeEvents: (params?: {
    page?: number;
    pageSize?: number;
    startTime?: string;
    endTime?: string;
    type?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
      if (params.type) queryParams.append('type', params.type);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/asset/change-events${query ? `?${query}` : ''}`));
  },

  getResourcePools: () => unwrap(apiRequest('/asset/resource-pools')),

  getCloudPlatforms: () => unwrap(apiRequest('/asset/cloud-platforms')),

  getRegions: (cloudPlatformId?: string) => {
    const params = new URLSearchParams();
    if (cloudPlatformId) params.append('cloud_platform_id', cloudPlatformId);
    const query = params.toString();
    return unwrap(apiRequest(`/asset/regions${query ? `?${query}` : ''}`));
  },

  getAvailabilityZones: (regionId?: string) => {
    const params = new URLSearchParams();
    if (regionId) params.append('region_id', regionId);
    const query = params.toString();
    return unwrap(apiRequest(`/asset/availability-zones${query ? `?${query}` : ''}`));
  },

  getServers: (params?: {
    page?: number;
    pageSize?: number;
    regionId?: string;
    zoneId?: string;
    status?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.regionId) queryParams.append('region_id', params.regionId);
      if (params.zoneId) queryParams.append('zone_id', params.zoneId);
      if (params.status) queryParams.append('status', params.status);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/asset/servers${query ? `?${query}` : ''}`));
  },

  getHosts: (params?: {
    page?: number;
    pageSize?: number;
    regionId?: string;
    zoneId?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.regionId) queryParams.append('region_id', params.regionId);
      if (params.zoneId) queryParams.append('zone_id', params.zoneId);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/asset/hosts${query ? `?${query}` : ''}`));
  },

  getVpcs: (params?: {
    page?: number;
    pageSize?: number;
    regionId?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.regionId) queryParams.append('region_id', params.regionId);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/asset/vpcs${query ? `?${query}` : ''}`));
  },

  getSubnets: (params?: {
    page?: number;
    pageSize?: number;
    vpcId?: string;
    regionId?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.vpcId) queryParams.append('vpc_id', params.vpcId);
      if (params.regionId) queryParams.append('region_id', params.regionId);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/asset/subnets${query ? `?${query}` : ''}`));
  },

  getRouters: (params?: {
    page?: number;
    pageSize?: number;
    vpcId?: string;
    regionId?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.vpcId) queryParams.append('vpc_id', params.vpcId);
      if (params.regionId) queryParams.append('region_id', params.regionId);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/asset/routers${query ? `?${query}` : ''}`));
  },

  getDhcpServers: (params?: {
    page?: number;
    pageSize?: number;
    vpcId?: string;
    regionId?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.vpcId) queryParams.append('vpc_id', params.vpcId);
      if (params.regionId) queryParams.append('region_id', params.regionId);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/asset/dhcp-servers${query ? `?${query}` : ''}`));
  },

  getIpAddresses: (params?: {
    page?: number;
    pageSize?: number;
    subnetId?: string;
    status?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.subnetId) queryParams.append('subnet_id', params.subnetId);
      if (params.status) queryParams.append('status', params.status);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/asset/ip-addresses${query ? `?${query}` : ''}`));
  },
};
