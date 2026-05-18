import { defineStore } from 'pinia';
import { api } from '../utils/api';
import type { Business, BusinessDetail, BusinessSearchParams, CreateBusinessRequest, UpdateBusinessRequest } from '../types';
import type { PaginatedResponse } from './index';

interface BusinessState {
  businesses: Business[];
  currentBusiness: BusinessDetail | null;
  loadingCount: number;
  error: string | null;
  // 分页信息
  total: Record<string, number>;
}

export const useBusinessStore = defineStore('business', {
  state: (): BusinessState => ({
    businesses: [],
    currentBusiness: null,
    loadingCount: 0,
    error: null,
    total: {}
  }),

  getters: {
    getBusinesses: (state) => state.businesses,
    getCurrentBusiness: (state) => state.currentBusiness,
    getLoading: (state) => state.loadingCount > 0,
    getError: (state) => state.error
  },

  actions: {
    async fetchBusinesses(params?: BusinessSearchParams): Promise<Business[]> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.business.getBusinesses(params);
        const paginated = response as unknown as PaginatedResponse<Business>;
        if (Array.isArray(paginated?.data) && typeof paginated.total === 'number') {
          this.businesses = paginated.data;
          this.total['businesses'] = paginated.total;
          return paginated.data;
        }
        const businesses = response as unknown as Business[];
        this.businesses = businesses;
        return businesses;
      } catch (e) {
        this.error = (e as Error).message || '获取业务列表失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async fetchBusinessDetail(id: string): Promise<BusinessDetail> {
      this.loadingCount++;

      try {
        const business = await api.business.getBusinessDetail(id);
        this.currentBusiness = business;
        return business;
      } catch (e) {
        this.error = (e as Error).message || '获取业务详情失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async createBusiness(data: CreateBusinessRequest): Promise<Business> {
      this.loadingCount++;

      try {
        const business = await api.business.createBusiness(data);
        this.businesses.push(business);
        return business;
      } catch (e) {
        this.error = (e as Error).message || '创建业务失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async updateBusiness(id: string, data: UpdateBusinessRequest): Promise<Business> {
      this.loadingCount++;

      try {
        const business = await api.business.updateBusiness(id, data);
        const index = this.businesses.findIndex(b => b.id === id);
        if (index !== -1) {
          this.businesses[index] = business;
          if (this.currentBusiness && this.currentBusiness.id === id) {
            this.currentBusiness = { ...this.currentBusiness, ...business };
          }
        }
        return business;
      } catch (e) {
        this.error = (e as Error).message || '更新业务失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    async deleteBusiness(id: string): Promise<void> {
      this.loadingCount++;

      try {
        await api.business.deleteBusiness(id);
        const index = this.businesses.findIndex(b => b.id === id);
        if (index !== -1) {
          this.businesses.splice(index, 1);
          if (this.currentBusiness && this.currentBusiness.id === id) {
            this.currentBusiness = null;
          }
        }
      } catch (e) {
        this.error = (e as Error).message || '删除业务失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    setCurrentBusiness(business: BusinessDetail | null) {
      this.currentBusiness = business;
    },

    clearCurrentBusiness() {
      this.currentBusiness = null;
    }
  }
});