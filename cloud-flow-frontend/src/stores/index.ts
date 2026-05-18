// 分页响应通用类型（与后端 API 返回的分页结构一致）
export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page?: number;
  page_size?: number;
}

// 从 api/index.ts 重新导出分页工具
export { unwrapPaginatedResponse } from '../api/index';
export type { PaginatedResponse as ApiPaginatedResponse } from '../api/index';

export * from './user';
export * from './system';
export * from './asset';
export * from './network';
export * from './business';
export * from './service';
