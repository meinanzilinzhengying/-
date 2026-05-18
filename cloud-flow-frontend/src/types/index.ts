export * from './user';
export * from './asset';
export * from './system';
export * from './network';
export * from './metrics';
export * from './alert';
export * from './report';
export * from './business';
export * from './service';
export * from './snapshot';

export interface PaginationParams {
  page?: number;
  pageSize?: number;
}

export interface TimeRangeParams {
  timeRange?: string;
  startTime?: string;
  endTime?: string;
}

export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data: T;
}

export interface ListResponse<T = unknown> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}