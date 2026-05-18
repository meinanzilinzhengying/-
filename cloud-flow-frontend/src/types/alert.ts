export interface Alert {
  id: string;
  level: string;
  title: string;
  message: string;
  source: string;
  service?: string;
  status: string;
  time: string;
  handler?: string;
  handleTime?: string;
  remark?: string;
}

export interface AlertRule {
  id: string;
  name: string;
  metric: string;
  condition: string;
  threshold: number;
  level: string;
  status: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface AlertSearchParams {
  page?: number;
  pageSize?: number;
  level?: string;
  status?: string;
  service?: string;
  startTime?: string;
  endTime?: string;
}

export interface AlertRuleSearchParams {
  page?: number;
  pageSize?: number;
  status?: string;
  level?: string;
}

export interface HandleAlertRequest {
  status: string;
  remark?: string;
}

export interface CreateAlertRuleRequest {
  name: string;
  metric: string;
  condition: string;
  threshold: number;
  level: string;
}

export interface UpdateAlertRuleRequest extends Partial<CreateAlertRuleRequest> {
  status?: string;
}