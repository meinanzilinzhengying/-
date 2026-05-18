export interface ChangeEvent {
  id: string;
  type: string;
  title: string;
  description: string;
  source: string;
  status: string;
  time: string;
  user?: string;
}

export interface ResourcePool {
  id: string;
  name: string;
  type: string;
  status: string;
  cloudPlatform?: string;
  region?: string;
  resourceCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface CloudPlatform {
  id: string;
  name: string;
  type: string;
  status: string;
  syncInterval?: number;
  lastSyncTime?: string;
  regionCount?: number;
  resourceCount?: number;
}

export interface Region {
  id: string;
  name: string;
  code: string;
  cloudPlatformId: string;
  status: string;
  availabilityZoneCount?: number;
}

export interface AvailabilityZone {
  id: string;
  name: string;
  code: string;
  regionId: string;
  status: string;
  resourceCount?: number;
}

export interface Server {
  id: string;
  name: string;
  ip: string;
  status: string;
  type: string;
  regionId?: string;
  zoneId?: string;
  vpcId?: string;
  subnetId?: string;
  cpu?: number;
  memory?: number;
  createdAt?: string;
}

export interface Host {
  id: string;
  name: string;
  ip: string;
  status: string;
  type: string;
  regionId?: string;
  zoneId?: string;
  cpu?: number;
  memory?: number;
  createdAt?: string;
}

export interface VPC {
  id: string;
  name: string;
  cidr: string;
  status: string;
  regionId: string;
  subnetCount?: number;
  instanceCount?: number;
}

export interface Subnet {
  id: string;
  name: string;
  cidr: string;
  vpcId: string;
  regionId: string;
  status: string;
  availableIpCount?: number;
}

export interface Router {
  id: string;
  name: string;
  status: string;
  vpcId: string;
  regionId: string;
  routeCount?: number;
}

export interface DhcpServer {
  id: string;
  name: string;
  ip: string;
  status: string;
  vpcId: string;
  regionId: string;
}

export interface IpAddress {
  id: string;
  address: string;
  type: string;
  status: string;
  subnetId: string;
  instanceId?: string;
  instanceName?: string;
}

export interface AssetSearchParams {
  page?: number;
  pageSize?: number;
  startTime?: string;
  endTime?: string;
  type?: string;
  regionId?: string;
  zoneId?: string;
  status?: string;
  vpcId?: string;
  subnetId?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}