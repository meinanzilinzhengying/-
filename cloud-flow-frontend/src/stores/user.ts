import { defineStore } from 'pinia';
import { api, resetRedirect } from '../utils/api';
import type { User, LoginResponse, UserUpdateRequest, ChangePasswordRequest } from '../types';

interface UserState {
  userInfo: User | null;
  isLoggedIn: boolean;
  loadingCount: number;
  error: string | null;
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    userInfo: null,
    isLoggedIn: false,
    loadingCount: 0,
    error: null
  }),

  getters: {
    getUserInfo: (state) => state.userInfo,
    getIsLoggedIn: (state) => state.isLoggedIn,
    getLoading: (state) => state.loadingCount > 0,
    getError: (state) => state.error
  },

  actions: {
    async login(username: string, password: string, rememberMe: boolean = false): Promise<LoginResponse> {
      this.loadingCount++;
      this.error = null;

      try {
        const response = await api.login(username, password, rememberMe);
        this.userInfo = response.user;
        this.isLoggedIn = true;
        resetRedirect();
        return response;
      } catch (e) {
        this.error = (e as Error).message || '登录失败';
        throw e;
      } finally {
        this.loadingCount--;
      }
    },

    logout() {
      api.logout();
      this.userInfo = null;
      this.isLoggedIn = false;
    },

    async checkAuth(): Promise<boolean> {
      this.loadingCount++;

      try {
        const isAuthenticated = await api.isAuthenticated();
        this.isLoggedIn = isAuthenticated;

        if (isAuthenticated) {
          await this.getUserInfoAction();
        }

        return isAuthenticated;
      } catch (e) {
        this.isLoggedIn = false;
        this.userInfo = null;
        return false;
      } finally {
        this.loadingCount--;
      }
    },

    async getUserInfoAction(): Promise<User> {
      try {
        const userInfo = await api.user.getUserInfo();
        this.userInfo = userInfo;
        return userInfo;
      } catch (e) {
 // console.error('获取用户信息失败:', e);
        throw e;
      }
    },

    async updateUserInfo(data: UserUpdateRequest): Promise<User> {
      try {
        const updatedUser = await api.user.updateUserInfo(data);
        this.userInfo = updatedUser;
        return updatedUser;
      } catch (e) {
 // console.error('更新用户信息失败:', e);
        throw e;
      }
    },

    async changePassword(oldPassword: string, newPassword: string): Promise<void> {
      try {
        await api.user.changePassword({ oldPassword, newPassword });
      } catch (e) {
 // console.error('修改密码失败:', e);
        throw e;
      }
    }
  }
});