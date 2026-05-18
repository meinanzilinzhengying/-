export interface ResourceAnalysis {
  id: string;
  type: string;
  name: string;
  value: number;
  unit?: string;
  time?: string;
}

export interface PathAnalysis {
  id: string;
  source: string;
  destination: string;
  protocol: string;
  hopCount: number;
  latency: number;
  packetLoss?: number;
  path?: string[];
}

export interface TopologyNode {
  id: string;
  name: string;
  type: string;
  ip?: string;
  port?: number;
  status: string;
  x?: number;
  y?: number;
  children?: TopologyNode[];
}

export interface TopologyEdge {
  id: string;
  source: string;
  target: string;
  type: string;
  latency?: number;
  bandwidth?: string;
}

export interface TopologyData {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

export interface FlowLog {
  id: string;
  sourceIp: string;
  sourcePort: number;
  destinationIp: string;
  destinationPort: number;
  protocol: string;
  bytes: number;
  packets: number;
  time: string;
  status: string;
}

export interface NetworkSearchParams {
  page?: number;
  pageSize?: number;
  timeRange?: string;
  startTime?: string;
  endTime?: string;
  source?: string;
  destination?: string;
  protocol?: string;
  resourceType?: string;
}

export interface FlowLogSearchParams extends NetworkSearchParams {
  status?: string;
}

export interface TopologyAnalysisParams {
  timeRange?: string;
  startTime?: string;
  endTime?: string;
  depth?: number;
  serviceType?: string;
}