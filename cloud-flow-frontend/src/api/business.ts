import { apiRequest, type ApiResponse } from './index';

// 解包 ApiResponse，返回 data 字段
function unwrap<T>(promise: Promise<ApiResponse<T>>): Promise<T> {
  return promise.then(r => r.data);
}

export const businessApi = {
  getBusinesses: (params?: {
    page?: number;
    pageSize?: number;
    status?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.status) queryParams.append('status', params.status);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/business${query ? `?${query}` : ''}`));
  },

  getBusinessDetail: (id: string) => unwrap(apiRequest(`/business/${id}`)),

  createBusiness: (data: Record<string, unknown>) => unwrap(apiRequest<Record<string, unknown>>('/business', {
    method: 'POST',
    body: data,
  })),

  updateBusiness: (id: string, data: Record<string, unknown>) => unwrap(apiRequest<Record<string, unknown>>(`/business/${id}`, {
    method: 'PUT',
    body: data,
  })),

  deleteBusiness: (id: string) => unwrap(apiRequest(`/business/${id}`, {
    method: 'DELETE',
  })),
};

export const serviceApi = {
  getServices: (params?: {
    page?: number;
    pageSize?: number;
    businessId?: string;
    status?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.businessId) queryParams.append('business_id', params.businessId);
      if (params.status) queryParams.append('status', params.status);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/service${query ? `?${query}` : ''}`));
  },

  getServiceDetail: (id: string) => unwrap(apiRequest(`/service/${id}`)),

  createService: (data: Record<string, unknown>) => unwrap(apiRequest<Record<string, unknown>>('/service', {
    method: 'POST',
    body: data,
  })),

  updateService: (id: string, data: Record<string, unknown>) => unwrap(apiRequest<Record<string, unknown>>(`/service/${id}`, {
    method: 'PUT',
    body: data,
  })),

  deleteService: (id: string) => unwrap(apiRequest(`/service/${id}`, {
    method: 'DELETE',
  })),
};
