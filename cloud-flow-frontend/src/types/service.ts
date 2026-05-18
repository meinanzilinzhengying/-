export interface Service {
  id: string;
  name: string;
  type: string;
  status: string;
  businessId?: string;
  description?: string;
  endpointCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface ServiceDetail extends Service {
  endpoints: Endpoint[];
  instances?: ServiceInstance[];
  metrics?: ServiceMetrics;
}

export interface Endpoint {
  id: string;
  url: string;
  method: string;
  status: string;
  latency?: number;
  requestCount?: number;
  errorRate?: number;
}

export interface ServiceInstance {
  id: string;
  name: string;
  ip: string;
  port: number;
  status: string;
  health?: string;
  uptime?: number;
}

export interface ServiceMetrics {
  totalRequests: number;
  successRate: number;
  avgLatency: number;
  p99Latency: number;
  rpm?: number;
}

export interface ServiceSearchParams {
  page?: number;
  pageSize?: number;
  businessId?: string;
  status?: string;
  type?: string;
}

export interface CreateServiceRequest {
  name: string;
  type: string;
  businessId?: string;
  description?: string;
}

export interface UpdateServiceRequest {
  name?: string;
  type?: string;
  status?: string;
  description?: string;
}