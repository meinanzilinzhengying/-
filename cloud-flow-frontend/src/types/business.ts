import type { Service, Endpoint } from './service'

export interface Business {
  id: string;
  name: string;
  description?: string;
  status: string;
  owner?: string;
  team?: string;
  table?: string;
  services?: { id: string; name: string; status: string }[];
  serviceGroups?: { id: string; name: string; filter?: string }[];
  paths?: number;
  creator?: string;
  serviceCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface BusinessDetail extends Business {
  services: Service[];
  endpoints?: Endpoint[];
  metrics?: BusinessMetrics;
}

export interface BusinessMetrics {
  totalRequests: number;
  successRate: number;
  avgLatency: number;
  p99Latency: number;
}

export interface BusinessSearchParams {
  page?: number;
  pageSize?: number;
  status?: string;
  owner?: string;
}

export interface CreateBusinessRequest {
  name: string;
  description?: string;
  owner?: string;
  team?: string;
  table?: string;
}

export interface UpdateBusinessRequest {
  name?: string;
  description?: string;
  status?: string;
  owner?: string;
  team?: string;
  table?: string;
}