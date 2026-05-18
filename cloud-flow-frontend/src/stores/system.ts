import { defineStore } from 'pinia';
import { api } from '../utils/api';
import type { Collector, DataNode, SystemConfig, SystemLog, CollectorSearchParams, DataNodeSearchParams, SystemLogSearchParams } from '../types';
import type { PaginatedResponse } from './index';

interface SystemState {
  collectors: Collector[];
  dataNodes: DataNode[];
  systemConfig: SystemConfig | null;
  systemLogs: SystemLog[];
  loadingCount: number;
  error: string | null;
  // 分页信息
  total: Record<string, number>;
}

export const useSystemStore = defineStore('system', {
  state: (): SystemState => ({
    collectors: [],
    dataNodes: [],
    systemConfig: null,
    systemLogs: [],
    loadingCount: 0,
    error: null,
    total: {}
  }),

  getters: {
    getCollectors: (state) => state.collectors,
    getDataNodes: (state) => state.dataNodes,
    getSystemConfig: (state) => state.systemConfig,
    getSystemLogs: (state) => state.systemLogs,
    getLoading: (state) => state.loadingCount > 0,
    getError: (state) => state.error
  },

  actions: {
    async fetchCollectors(params?: CollectorSearchParams): Promise<Collector[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.system.getCollectors(params);
        const paginated = response as unknown as PaginatedResponse<Collector>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.collectors = paginated.data;
          this.total['collectors'] = paginated.total;
          return paginated.data;
        }
        const collectors = response as unknown as Collector[];
        this.collectors = collectors;
        return collectors;
      } catch (e) {
        this.error = (e as Error).message || '获取采集器列表失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchCollectorDetail(id: string): Promise<Collector> {
      this.loadingCount++;

      try {
        const collector = await api.system.getCollectorDetail(id);
        return collector;
      } catch (e) {
        this.error = (e as Error).message || '获取采集器详情失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async startCollector(id: string): Promise<void> {
      this.loadingCount++;

      try {
        await api.system.startCollector(id);
        await this.fetchCollectors();
      } catch (e) {
        this.error = (e as Error).message || '启动采集器失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async stopCollector(id: string): Promise<void> {
      this.loadingCount++;

      try {
        await api.system.stopCollector(id);
        await this.fetchCollectors();
      } catch (e) {
        this.error = (e as Error).message || '停止采集器失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async restartCollector(id: string): Promise<void> {
      this.loadingCount++;

      try {
        await api.system.restartCollector(id);
        await this.fetchCollectors();
      } catch (e) {
        this.error = (e as Error).message || '重启采集器失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchDataNodes(params?: DataNodeSearchParams): Promise<DataNode[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.system.getDataNodes(params);
        const paginated = response as unknown as PaginatedResponse<DataNode>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.dataNodes = paginated.data;
          this.total['dataNodes'] = paginated.total;
          return paginated.data;
        }
        const dataNodes = response as unknown as DataNode[];
        this.dataNodes = dataNodes;
        return dataNodes;
      } catch (e) {
        this.error = (e as Error).message || '获取数据库节点列表失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchDataNodeDetail(id: string): Promise<DataNode> {
      this.loadingCount++;

      try {
        const dataNode = await api.system.getDataNodeDetail(id);
        return dataNode;
      } catch (e) {
        this.error = (e as Error).message || '获取数据库节点详情失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchSystemConfig(): Promise<SystemConfig> {
      this.loadingCount++;

      try {
        const config = await api.system.getSystemConfig();
        this.systemConfig = config;
        return config;
      } catch (e) {
        this.error = (e as Error).message || '获取系统配置失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async updateSystemConfig(config: SystemConfig): Promise<SystemConfig> {
      this.loadingCount++;

      try {
        const updatedConfig = await api.system.updateSystemConfig(config);
        this.systemConfig = updatedConfig;
        return updatedConfig;
      } catch (e) {
        this.error = (e as Error).message || '更新系统配置失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchSystemLogs(params?: SystemLogSearchParams): Promise<SystemLog[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.system.getSystemLogs(params);
        const paginated = response as unknown as PaginatedResponse<SystemLog>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.systemLogs = paginated.data;
          this.total['systemLogs'] = paginated.total;
          return paginated.data;
        }
        const logs = response as unknown as SystemLog[];
        this.systemLogs = logs;
        return logs;
      } catch (e) {
        this.error = (e as Error).message || '获取系统日志失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    }
  }
});