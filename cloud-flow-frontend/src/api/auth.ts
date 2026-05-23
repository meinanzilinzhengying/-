import { apiRequest, setToken, removeToken, getToken, getCSRFToken, type ApiResponse } from './index';

// 解包 ApiResponse，返回 data 字段
function unwrap<T>(promise: Promise<ApiResponse<T>>): Promise<T> {
  return promise.then(r => r.data);
}

export const authApi = {
  login: async (username: string, password: string, rememberMe: boolean = false) => {
    // 预获取 CSRF token（通过 GET 请求让后端设置 csrf_token cookie，
    // 同时从响应头 X-CSRF-Token 获取 token 用于后续请求头提交）
    try {
      await apiRequest('/csrf-token', { method: 'GET' }, 0);
    } catch {
      // 即使失败也继续尝试登录
    }
    // POST 登录（自动携带 X-CSRF-Token 头）
    const response = await apiRequest<{ token: string; user: unknown }>('/users/login', {
      method: 'POST',
      body: { username, password },
    }, 0);
    // 兼容两种响应格式：直接返回 token 或包裹在 data 中
    const token = (response as Record<string, unknown>).token
      || (response as Record<string, unknown>).data
        ? ((response as Record<string, unknown>).data as Record<string, unknown>).token
        : null;
    if (token) {
      await setToken(token as string, rememberMe);
    }
    return response;
  },

  logout: () => {
    removeToken();
  },

  isAuthenticated: async () => {
    const token = await getToken();
    if (!token) {
      return false;
    }

    try {
      await apiRequest('/users/verify');
      return true;
    } catch (e) {
      removeToken();
      return false;
    }
  },
};

export const userApi = {
  getUserInfo: () => unwrap(apiRequest('/users/info')),

  updateUserInfo: (data: Record<string, unknown>) => unwrap(apiRequest<Record<string, unknown>>('/users/info', {
    method: 'PUT',
    body: data,
  })),

  changePassword: (data: {
    oldPassword: string;
    newPassword: string;
  }) => unwrap(apiRequest('/users/password', {
    method: 'PUT',
    body: data,
  })),

  getUsers: (params?: {
    page?: number;
    pageSize?: number;
    role?: string;
    status?: string;
  }) => {
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.append('page', params.page.toString());
      if (params.pageSize) queryParams.append('page_size', params.pageSize.toString());
      if (params.role) queryParams.append('role', params.role);
      if (params.status) queryParams.append('status', params.status);
    }
    const query = queryParams.toString();
    return unwrap(apiRequest(`/users${query ? `?${query}` : ''}`));
  },

  createUser: (data: Record<string, unknown>) => unwrap(apiRequest<Record<string, unknown>>('/users', {
    method: 'POST',
    body: data,
  })),

  updateUser: (id: string, data: Record<string, unknown>) => unwrap(apiRequest<Record<string, unknown>>(`/users/${id}`, {
    method: 'PUT',
    body: data,
  })),

  deleteUser: (id: string) => unwrap(apiRequest(`/users/${id}`, {
    method: 'DELETE',
  })),
};
