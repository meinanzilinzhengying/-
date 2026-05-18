import { apiRequest } from './index';

// 告警规则条件（与后端 alerting.Condition 结构一致）
export interface AlertCondition {
  metric: string;
  operator: string;  // '>', '<', '>=', '<=', '=', '!='
  threshold: number;
}

// 告警规则（与后端 alerting.Rule 结构一致）
export interface AlertRule {
  id: string;
  name: string;
  description: string;
  type: string;       // 'cpu', 'memory', 'network', 'disk', 'traffic'
  enabled: boolean;
  condition: AlertCondition;
  threshold: number;
  duration: number;   // 持续时间（秒）
  severity: string;   // 'critical', 'warning', 'info'
  labels: Record<string, string>;
  satisfy_threshold: number;
  created_at: string;
  updated_at: string;
}

// 告警信息（与后端 alerting.Alert 结构一致）
export interface Alert {
  id: string;
  rule_id: string;
  rule_name: string;
  severity: string;
  message: string;
  labels: Record<string, string>;
  value: number;
  threshold: number;
  created_at: string;
  resolved: boolean;
  resolved_at: string;
}

export const alertsApi = {
  getAlerts: () => apiRequest('/alerts'),

  getRules: () => apiRequest('/rules'),

  getAlertsList: (params?: {
    page?: number;
    pageSize?: number;
    level?: string;
    status?: string;
    service?: string;
    startTime?: string;
    endTime?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.level) queryParams.append('level', params.level);
      if (params.status) queryParams.append('status', params.status);
      if (params.service) queryParams.append('service', params.service);
      if (params.startTime) queryParams.append('start_time', params.startTime);
      if (params.endTime) queryParams.append('end_time', params.endTime);
    }
    const query = queryParams.toString();
    return apiRequest(`/alert/list${query ? `?${query}` : ''}`);
  },

  getAlertDetail: (id: string) => apiRequest(`/alert/${id}`),

  handleAlert: (id: string, data: {
    status: string;
    remark?: string;
  }) => apiRequest(`/alert/${id}/handle`, {
    method: 'PUT',
    body: data,
  }),

  getAlertRules: (params?: {
    page?: number;
    pageSize?: number;
    status?: string;
    level?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.status) queryParams.append('status', params.status);
      if (params.level) queryParams.append('level', params.level);
    }
    const query = queryParams.toString();
    return apiRequest(`/alert/rules${query ? `?${query}` : ''}`);
  },

  createAlertRule: (data: Record<string, unknown>) => apiRequest('/alert/rules', {
    method: 'POST',
    body: data,
  }),

  updateAlertRule: (id: string, data: Record<string, unknown>) => apiRequest(`/alert/rules/${id}`, {
    method: 'PUT',
    body: data,
  }),

  deleteAlertRule: (id: string) => apiRequest(`/alert/rules/${id}`, {
    method: 'DELETE',
  }),
};
