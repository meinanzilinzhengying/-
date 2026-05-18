import { ElMessage } from 'element-plus';
import router, { clearAuthCache } from '../router';

// ============================================================
// API 基础配置
// ============================================================

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api';

const TOKEN_KEY = 'cf_token';

// =============================================================================
// [安全警告] Token 存储安全说明
// =============================================================================
// 当前实现使用 localStorage 存储 JWT Token，这在开发环境中可以接受，
// 但在生产环境中存在以下安全风险：
//
// 1. XSS 攻击风险：localStorage 可被 JavaScript 访问，如果应用存在 XSS 漏洞，
//    攻击者可以通过恶意脚本读取 Token。
// 2. Token 无法自动过期：localStorage 中的数据不会自动过期，需要手动清理。
// 3. 敏感信息暴露：Token 会一直保留在浏览器中，即使用户关闭了标签页。
//
// 【生产环境建议】：
// - 使用 httpOnly Cookie 存储 Token，防止 JavaScript 访问
// - 配合 SameSite=Strict/Lax 属性防止 CSRF 攻击
// - 在 HTTPS 环境下设置 Secure 属性
// - 后端应在 Set-Cookie 响应头中设置合理的 Max-Age 和 Path
//
// 如需改造，建议方案：
//   1. 后端登录接口改为 Set-Cookie 方式返回 Token
//   2. 前端移除 localStorage 存取 Token 的逻辑
//   3. Cookie 设置 httpOnly=true; Secure=true; SameSite=Lax; Path=/
// =============================================================================

// ============================================================
// AES-GCM Token 加密（增强安全性）
// ============================================================

/** Derive device key from browser fingerprint */
async function deriveDeviceKey(): Promise<CryptoKey> {
  const material = [
    navigator.userAgent,
    `${screen.width}x${screen.height}`,
    Intl.DateTimeFormat().resolvedOptions().timeZone,
  ].join('|');
  const encoder = new TextEncoder();
  const rawKey = await crypto.subtle.digest('SHA-256', encoder.encode(material));
  return crypto.subtle.importKey('raw', rawKey, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt']);
}

/** AES-GCM encrypt token */
async function encryptToken(token: string): Promise<string> {
  const key = await deriveDeviceKey();
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const encoder = new TextEncoder();
  const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, encoder.encode(token));
  const combined = new Uint8Array(iv.length + ciphertext.byteLength);
  combined.set(iv, 0);
  combined.set(new Uint8Array(ciphertext), iv.length);
  return btoa(String.fromCharCode(...combined));
}

/** AES-GCM decrypt token */
async function decryptToken(encoded: string): Promise<string | null> {
  try {
    const key = await deriveDeviceKey();
    const combined = Uint8Array.from(atob(encoded), c => c.charCodeAt(0));
    const iv = combined.slice(0, 12);
    const ciphertext = combined.slice(12);
    const plaintext = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ciphertext);
    return new TextDecoder().decode(plaintext);
  } catch { return null; }
}

// 401 并发请求去重标志位
let isRedirecting = false;

// 创建 AbortController 工具函数（供组件在卸载时取消请求）
export const createAbortController = (): AbortController => new AbortController();

// 重置 isRedirecting 标志位（登录成功后调用）
export const resetRedirect = () => {
  isRedirecting = false;
};

export const getToken = async (): Promise<string | null> => {
  const encoded = localStorage.getItem(TOKEN_KEY);
  if (!encoded) return null;
  return decryptToken(encoded);
};

export const setToken = async (token: string, rememberMe: boolean = false): Promise<void> => {
  const encrypted = await encryptToken(token);
  localStorage.setItem(TOKEN_KEY, encrypted);
  // 记住我：将记住状态存储到 localStorage，供后续判断 token 过期策略使用
  if (rememberMe) {
    localStorage.setItem('cf_remember_me', 'true');
  } else {
    localStorage.removeItem('cf_remember_me');
  }
};

export const removeToken = (): void => {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem('cf_remember_me');
};

// ============================================================
// 通用响应包装类型
// ============================================================

export interface ApiResponse<T> {
  success: boolean;
  data: T;
  message?: string;
}

// ============================================================
// 分页响应类型与工具函数
// ============================================================

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page?: number;
  page_size?: number;
}

export function unwrapPaginatedResponse<T>(response: unknown): { items: T[]; total: number } {
  const paginated = response as PaginatedResponse<T>;
  if (paginated && typeof paginated === 'object' && Array.isArray(paginated.data) && typeof paginated.total === 'number') {
    return { items: paginated.data, total: paginated.total };
  }
  const items = Array.isArray(response) ? response : [];
  return { items, total: 0 };
}

// ============================================================
// 通用请求函数
// ============================================================

const DEFAULT_TIMEOUT = 30000; // 30 seconds

interface ApiRequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  headers?: Record<string, string>;
  body?: Record<string, unknown>;
  signal?: AbortSignal;
  timeout?: number;
}

export async function apiRequest<T>(
  endpoint: string,
  options: ApiRequestOptions = {},
  retryCount: number | null = null,
  retryDelay: number = 1000
): Promise<ApiResponse<T>> {
  const { method = 'GET', headers = {}, body, signal, timeout } = options;

  if (retryCount === null) {
    if (method === 'GET') {
      retryCount = 3;
    } else {
      retryCount = 0;
    }
  }

  const timeoutMs = timeout ?? DEFAULT_TIMEOUT;
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  if (signal) {
    signal.addEventListener('abort', () => controller.abort());
  }

  const token = await getToken();

  const config: RequestInit = {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      ...headers,
    },
    signal: controller.signal,
  };

  if (body && method !== 'GET') {
    config.body = JSON.stringify(body);
  }

  try {
    const response = await fetch(`${API_BASE_URL}${endpoint}`, config);

    if (!response.ok) {
      switch (response.status) {
        case 401:
          if (isRedirecting) return Promise.reject(new Error('Unauthorized'));
          isRedirecting = true;
          ElMessage.error('登录已过期，请重新登录');
          removeToken();
          clearAuthCache();
          router.push('/login');
          break;
        case 403:
          ElMessage.error('没有权限访问此资源');
          break;
        case 404:
          ElMessage.error('请求的资源不存在');
          break;
        case 500:
          ElMessage.error('服务器内部错误，请稍后重试');
          break;
        default:
          ElMessage.error(`请求失败 (${response.status})`);
      }
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const jsonData = await response.json();
    // 如果后端返回的已经是 ApiResponse 格式（包含 success 字段），直接返回
    if (jsonData && typeof jsonData === 'object' && 'success' in jsonData) {
      return jsonData as ApiResponse<T>;
    }
    // 否则包装为统一的 ApiResponse 格式
    return { success: true, data: jsonData as T };
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') {
      throw new Error(`请求超时 (${timeoutMs / 1000}s)`);
    }
    if (retryCount > 0 && (e instanceof TypeError || (e as Error).message?.includes('Network'))) {
      await new Promise(resolve => setTimeout(resolve, retryDelay));
      return apiRequest<T>(endpoint, options, retryCount - 1, retryDelay * 2);
    }

    throw e;
  } finally {
    clearTimeout(timeoutId);
  }
}

// ============================================================
// 统一导出兼容旧 api 对象（保持向后兼容）
// ============================================================

export { API_BASE_URL };

// 导出各模块 API
export { authApi, userApi } from './auth';
export { metricsApi } from './metrics';
export { alertsApi } from './alerts';
export { assetsApi } from './assets';
export { networkApi } from './network';
export { businessApi, serviceApi } from './business';
export { systemApi, reportApi } from './system';

// 兼容旧的 api 对象导出，确保现有引用无需修改
import { authApi, userApi } from './auth';
import { metricsApi } from './metrics';
import { alertsApi } from './alerts';
import { assetsApi } from './assets';
import { networkApi } from './network';
import { businessApi, serviceApi } from './business';
import { systemApi, reportApi } from './system';

export const api = {
  // 仪表盘概览
  getOverview: metricsApi.getOverview,
  getNodes: metricsApi.getNodes,
  getTraffic: metricsApi.getTraffic,
  getProtocol: metricsApi.getProtocol,
  getCpu: metricsApi.getCpu,
  getMemory: metricsApi.getMemory,
  getMetrics: metricsApi.getMetrics,
  getAlerts: alertsApi.getAlerts,
  getRules: alertsApi.getRules,

  // 链路追踪 & 拓扑
  getTracing: networkApi.getTracing,
  getTopology: networkApi.getTopology,

  // 认证
  login: authApi.login,
  logout: authApi.logout,
  isAuthenticated: authApi.isAuthenticated,

  // 资产管理
  asset: assetsApi,

  // 系统管理
  system: systemApi,

  // 网络分析
  network: networkApi,

  // 指标数据
  metrics: {
    getMetricList: metricsApi.getMetricList,
    getMetricDetail: metricsApi.getMetricDetail,
    getMetricTrend: metricsApi.getMetricTrend,
    getMetricAggregation: metricsApi.getMetricAggregation,
  },

  // 报告
  report: reportApi,

  // 告警
  alert: alertsApi,

  // 用户管理
  user: userApi,

  // 业务观测
  business: businessApi,

  // 服务管理
  service: serviceApi,
};
