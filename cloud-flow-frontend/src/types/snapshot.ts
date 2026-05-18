export interface Snapshot {
  id: number;
  name: string;
  description: string;
  usageCount: number;
  createTime: string;
  starred: boolean;
  isDefault: boolean;
}

export interface SearchTag {
  key: string;
  operator: string;
  value: string;
}

export interface SearchResult {
  id: number;
  time: string;
  message: string;
  level: string;
}

export interface SearchHistory {
  id: number;
  query: string;
  time: string;
}

export interface SearchParams {
  timeRange?: string;
  startTime?: string;
  endTime?: string;
  tags?: SearchTag[];
}