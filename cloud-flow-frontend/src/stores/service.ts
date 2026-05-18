import { defineStore } from 'pinia';
import { api } from '../utils/api';
import type { Service, ServiceDetail, ServiceSearchParams, CreateServiceRequest, UpdateServiceRequest } from '../types';
import type { PaginatedResponse } from './index';

interface ServiceState {
  services: Service[];
  currentService: ServiceDetail | null;
  loadingCount: number;
  error: string | null;
  // 分页信息
  total: Record<string, number>;
}

export const useServiceStore = defineStore('service', {
  state: (): ServiceState => ({
    services: [],
    currentService: null,
    loadingCount: 0,
    error: null,
    total: {}
  }),

  getters: {
    getServices: (state) => state.services,
    getCurrentService: (state) => state.currentService,
    getLoading: (state) => state.loadingCount > 0,
    getError: (state) => state.error
  },

  actions: {
    async fetchServices(params?: ServiceSearchParams): Promise<Service[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.service.getServices(params);
        const paginated = response as unknown as PaginatedResponse<Service>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.services = paginated.data;
          this.total['services'] = paginated.total;
          return paginated.data;
        }
        const services = response as unknown as Service[];
        this.services = services;
        return services;
      } catch (e) {
        this.error = (e as Error).message || '获取服务列表失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchServiceDetail(id: string): Promise<ServiceDetail> {
      this.loadingCount++;

      try {
        const service = await api.service.getServiceDetail(id);
        this.currentService = service;
        return service;
      } catch (e) {
        this.error = (e as Error).message || '获取服务详情失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async createService(data: CreateServiceRequest): Promise<Service> {
      this.loadingCount++;

      try {
        const service = await api.service.createService(data);
        this.services.push(service);
        return service;
      } catch (e) {
        this.error = (e as Error).message || '创建服务失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async updateService(id: string, data: UpdateServiceRequest): Promise<Service> {
      this.loadingCount++;

      try {
        const service = await api.service.updateService(id, data);
        const index = this.services.findIndex(s => s.id === id);
        if (index !== -1) {
          this.services[index] = service;
          if (this.currentService && this.currentService.id === id) {
            this.currentService = { ...this.currentService, ...service };
          }
        }
        return service;
      } catch (e) {
        this.error = (e as Error).message || '更新服务失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async deleteService(id: string): Promise<void> {
      this.loadingCount++;

      try {
        await api.service.deleteService(id);
        const index = this.services.findIndex(s => s.id === id);
        if (index !== -1) {
          this.services.splice(index, 1);
          if (this.currentService && this.currentService.id === id) {
            this.currentService = null;
          }
        }
      } catch (e) {
        this.error = (e as Error).message || '删除服务失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    setCurrentService(service: ServiceDetail | null) {
      this.currentService = service;
    },

    clearCurrentService() {
      this.currentService = null;
    }
  }
});