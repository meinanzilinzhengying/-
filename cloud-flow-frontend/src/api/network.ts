import { apiRequest, type ApiResponse } from './index';

// 解包 ApiResponse，返回 data 字段
function unwrap<T>(promise: Promise<ApiResponse<T>>): Promise<T> {
  return promise.then(r => r.data);
}

export const networkApi = {
  getTracing: (params?: {
    timeRange?: string;
    startTime?: string;
    endTime?: string;
    traceId?: string;
    serviceName?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.timeRange) queryParams.append('time_range', params.timeRange);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
      if (params.traceId) queryParams.append('trace_id', params.traceId);
      if (params.serviceName) queryParams.append('service_name', params.serviceName);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/tracing${query ? `?${query}` : ''}`));
  },

  getTopology: (params?: {
    timeRange?: string;
    startTime?: string;
    endTime?: string;
    cluster?: string;
    namespace?: string;
    serviceType?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.timeRange) queryParams.append('time_range', params.timeRange);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
      if (params.cluster) queryParams.append('cluster', params.cluster);
      if (params.namespace) queryParams.append('namespace', params.namespace);
      if (params.serviceType) queryParams.append('service_type', params.serviceType);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/topology${query ? `?${query}` : ''}`));
  },

  getResourceAnalysis: (params?: {
    timeRange?: string;
    startTime?: string;
    endTime?: string;
    resourceType?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.timeRange) queryParams.append('time_range', params.timeRange);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
      if (params.resourceType) queryParams.append('resource_type', params.resourceType);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/network/resource-analysis${query ? `?${query}` : ''}`));
  },

  getPathAnalysis: (params?: {
    source?: string;
    destination?: string;
    protocol?: string;
    startTime?: string;
    endTime?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.source) queryParams.append('source', params.source);
      if (params.destination) queryParams.append('destination', params.destination);
      if (params.protocol) queryParams.append('protocol', params.protocol);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/network/path-analysis${query ? `?${query}` : ''}`));
  },

  getTopologyAnalysis: (params?: {
    timeRange?: string;
    startTime?: string;
    endTime?: string;
    depth?: number;
    serviceType?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.timeRange) queryParams.append('time_range', params.timeRange);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
      if (params.depth) queryParams.append('depth', params.depth.toString());
      if (params.serviceType) queryParams.append('service_type', params.serviceType);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/network/topology-analysis${query ? `?${query}` : ''}`));
  },

  getFlowLogs: (params?: {
    page?: number;
    pageSize?: number;
    source?: string;
    destination?: string;
    protocol?: string;
    startTime?: string;
    endTime?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.source) queryParams.append('source', params.source);
      if (params.destination) queryParams.append('destination', params.destination);
      if (params.protocol) queryParams.append('protocol', params.protocol);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/network/flow-logs${query ? `?${query}` : ''}`));
  },
};
