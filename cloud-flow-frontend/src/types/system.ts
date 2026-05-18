export interface Collector {
  id: string;
  name: string;
  ip: string;
  status: string;
  type: string;
  version?: string;
  lastHeartbeat?: string;
  cpuUsage?: number;
  memoryUsage?: number;
  uptime?: number;
  createdAt?: string;
}

export interface DataNode {
  id: string;
  name: string;
  ip: string;
  status: string;
  type: string;
  version?: string;
  diskUsage?: number;
  cpuUsage?: number;
  memoryUsage?: number;
  lastHeartbeat?: string;
}

export interface SystemConfig {
  id: string;
  key: string;
  value: string;
  description?: string;
  type?: string;
  updatedAt?: string;
}

export interface SystemLog {
  id: string;
  level: string;
  message: string;
  source: string;
  time: string;
  details?: string;
}

export interface CollectorSearchParams {
  page?: number;
  pageSize?: number;
  status?: string;
}

export interface DataNodeSearchParams {
  page?: number;
  pageSize?: number;
  status?: string;
}

export interface SystemLogSearchParams {
  page?: number;
  pageSize?: number;
  level?: string;
  startTime?: string;
  endTime?: string;
}

export interface SystemOverview {
  totalCollectors: number;
  onlineCollectors: number;
  totalDataNodes: number;
  onlineDataNodes: number;
  totalTraffic: string;
  alerts: number;
  cpuUsage: number;
  memoryUsage: number;
}