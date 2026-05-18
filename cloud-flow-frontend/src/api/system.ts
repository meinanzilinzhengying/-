import { apiRequest, getToken, API_BASE_URL, type ApiResponse } from './index';

// 解包 ApiResponse，返回 data 字段
function unwrap<T>(promise: Promise<ApiResponse<T>>): Promise<T> {
  return promise.then(r => r.data);
}

export const systemApi = {
  getCollectors: (params?: {
    page?: number;
    pageSize?: number;
    status?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.status) queryParams.append('status', params.status);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/system/collectors${query ? `?${query}` : ''}`));
  },

  getCollectorDetail: (id: string) => unwrap(apiRequest(`/system/collectors/${id}`)),

  startCollector: (id: string) => unwrap(apiRequest(`/system/collectors/${id}/start`, { method: 'POST' })),

  stopCollector: (id: string) => unwrap(apiRequest(`/system/collectors/${id}/stop`, { method: 'POST' })),

  restartCollector: (id: string) => unwrap(apiRequest(`/system/collectors/${id}/restart`, { method: 'POST' })),

  getDataNodes: (params?: {
    page?: number;
    pageSize?: number;
    status?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.status) queryParams.append('status', params.status);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/system/data-nodes${query ? `?${query}` : ''}`));
  },

  getDataNodeDetail: (id: string) => unwrap(apiRequest(`/system/data-nodes/${id}`)),

  getSystemConfig: () => unwrap(apiRequest('/system/config')),

  updateSystemConfig: (config: Record<string, unknown>) => unwrap(apiRequest<Record<string, unknown>>('/system/config', {
    method: 'PUT',
    body: config,
  })),

  getSystemLogs: (params?: {
    page?: number;
    pageSize?: number;
    level?: string;
    startTime?: string;
    endTime?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.level) queryParams.append('level', params.level);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/system/logs${query ? `?${query}` : ''}`));
  },
};

export const reportApi = {
  getReports: (params?: {
    page?: number;
    pageSize?: number;
    type?: string;
    status?: string;
    startTime?: string;
    endTime?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.type) queryParams.append('type', params.type);
      if (params.status) queryParams.append('status', params.status);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/report/list${query ? `?${query}` : ''}`));
  },

  getReportDetail: (id: string) => unwrap(apiRequest(`/report/${id}`)),

  generateReport: (data: {
    type: string;
    title: string;
    startTime: string;
    endTime: string;
    params?: Record<string, unknown>;
  }) => unwrap(apiRequest('/report/generate', {
    method: 'POST',
    body: data,
  })),

  downloadReport: (id: string) => {
    const token = getToken();
    const headers = token ? { 'Authorization': `Bearer ${token}` } : {};
    return fetch(`${API_BASE_URL}/report/${id}/download`, {
      method: 'GET',
      headers,
    });
  },
};
