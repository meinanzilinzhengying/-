import { defineStore } from 'pinia';
import { api } from '../utils/api';
import type { ResourceAnalysis, PathAnalysis, TopologyData, FlowLog, NetworkSearchParams, FlowLogSearchParams, TopologyAnalysisParams } from '../types';
import type { PaginatedResponse } from './index';

interface NetworkState {
  resourceAnalysis: ResourceAnalysis | null;
  pathAnalysis: PathAnalysis | null;
  topologyAnalysis: TopologyData | null;
  flowLogs: FlowLog[];
  loadingCount: number;
  error: string | null;
  // 分页信息
  total: Record<string, number>;
}

export const useNetworkStore = defineStore('network', {
  state: (): NetworkState => ({
    resourceAnalysis: null,
    pathAnalysis: null,
    topologyAnalysis: null,
    flowLogs: [],
    loadingCount: 0,
    error: null,
    total: {}
  }),

  getters: {
    getResourceAnalysis: (state) => state.resourceAnalysis,
    getPathAnalysis: (state) => state.pathAnalysis,
    getTopologyAnalysis: (state) => state.topologyAnalysis,
    getFlowLogs: (state) => state.flowLogs,
    getLoading: (state) => state.loadingCount > 0,
    getError: (state) => state.error
  },

  actions: {
    async fetchResourceAnalysis(params?: NetworkSearchParams): Promise<ResourceAnalysis> {
      this.loadingCount++;
      this.error = null;

      try {
        const analysis = await api.network.getResourceAnalysis(params);
        this.resourceAnalysis = analysis;
        return analysis;
      } catch (e) {
        this.error = (e as Error).message || '获取资源分析失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchPathAnalysis(params?: NetworkSearchParams): Promise<PathAnalysis> {
      this.loadingCount++;
      this.error = null;

      try {
        const analysis = await api.network.getPathAnalysis(params);
        this.pathAnalysis = analysis;
        return analysis;
      } catch (e) {
        this.error = (e as Error).message || '获取路径分析失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchTopologyAnalysis(params?: TopologyAnalysisParams): Promise<TopologyData> {
      this.loadingCount++;
      this.error = null;

      try {
        const analysis = await api.network.getTopologyAnalysis(params);
        this.topologyAnalysis = analysis;
        return analysis;
      } catch (e) {
        this.error = (e as Error).message || '获取拓扑分析失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchFlowLogs(params?: FlowLogSearchParams): Promise<FlowLog[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.network.getFlowLogs(params);
        const paginated = response as unknown as PaginatedResponse<FlowLog>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.flowLogs = paginated.data;
          this.total['flowLogs'] = paginated.total;
          return paginated.data;
        }
        const logs = response as unknown as FlowLog[];
        this.flowLogs = logs;
        return logs;
      } catch (e) {
        this.error = (e as Error).message || '获取流日志失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    }
  }
});