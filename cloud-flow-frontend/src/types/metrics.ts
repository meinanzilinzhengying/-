export interface Metric {
  id: string;
  name: string;
  type: string;
  unit: string;
  category: string;
  probeId?: string;
  value?: number;
  timestamp?: string;
}

export interface MetricTrend {
  name: string;
  data: Array<{
    time: string;
    value: number;
  }>;
}

export interface MetricAggregation {
  name: string;
  value: number;
  aggregation: 'sum' | 'avg' | 'max' | 'min';
}

export interface MetricSearchParams {
  page?: number;
  pageSize?: number;
  category?: string;
  probeId?: string;
}

export interface MetricTrendParams {
  metricName: string;
  probeId?: string;
  timeRange?: string;
  startTime?: string;
  endTime?: string;
}

export interface MetricAggregationParams {
  metricName: string;
  aggregation: 'sum' | 'avg' | 'max' | 'min';
  probeId?: string;
  timeRange?: string;
  startTime?: string;
  endTime?: string;
}

export interface Overview {
  totalTraffic: string;
  totalPackets: number;
  totalConnections: number;
  activeProbes: number;
  alerts: number;
  cpuUsage: number;
  memoryUsage: number;
}