import { request } from './http';

export interface LoginResponse {
  access_token: string;
}

export interface CurrentAdmin {
  username: string;
}

export function login(username: string, password: string) {
  return request<LoginResponse>('/admin/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export function me() {
  return request<CurrentAdmin>('/admin/auth/me');
}
