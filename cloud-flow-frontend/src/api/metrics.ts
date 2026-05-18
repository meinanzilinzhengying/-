import { apiRequest } from './index';

export const metricsApi = {
  getOverview: () => apiRequest('/overview'),

  getNodes: () => apiRequest('/nodes'),

  getTraffic: () => apiRequest('/traffic'),

  getProtocol: () => apiRequest('/protocol'),

  getCpu: () => apiRequest('/cpu'),

  getMemory: () => apiRequest('/memory'),

  getMetrics: (date?: string, probeId?: string) => {
    const params = new URLSearchParams();
    if (date) params.append('date', date);
    if (probeId) params.append('probe_id', probeId);
    const query = params.toString();
    return apiRequest(`/metrics${query ? `?${query}` : ''}`);
  },

  getMetricList: (params?: {
    page?: number;
    pageSize?: number;
    category?: string;
    probeId?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.category) queryParams.append('category', params.category);
      if (params.probeId) queryParams.append('probe_id', params.probeId);
    }
    const query = queryParams.toString();
    return apiRequest(`/metrics/list${query ? `?${query}` : ''}`);
  },

  getMetricDetail: (id: string) => apiRequest(`/metrics/${id}`),

  getMetricTrend: (params: {
    metricName: string;
    probeId?: string;
    timeRange?: string;
    startTime?: string;
    endTime?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      queryParams.append('metric_name', params.metricName);
      if (params.probeId) queryParams.append('probe_id', params.probeId);
      if (params.timeRange) queryParams.append('time_range', params.timeRange);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
    }
    const query = queryParams.toString();
    return apiRequest(`/metrics/trend?${query}`);
  },

  getMetricAggregation: (params: {
    metricName: string;
    aggregation: 'sum' | 'avg' | 'max' | 'min';
    probeId?: string;
    timeRange?: string;
    startTime?: string;
    endTime?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      queryParams.append('metric_name', params.metricName);
      queryParams.append('aggregation', params.aggregation);
      if (params.probeId) queryParams.append('probe_id', params.probeId);
      if (params.timeRange) queryParams.append('time_range', params.timeRange);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
    }
    const query = queryParams.toString();
    return apiRequest(`/metrics/aggregation?${query}`);
  },
};
