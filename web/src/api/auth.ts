import { request } from './http';

export interface LoginResult {
  data: { access_token: string };
}

export interface MeResult {
  data: { username: string };
}

export function login(username: string, password: string) {
  return request<LoginResult>('/admin/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export function me() {
  return request<MeResult>('/admin/auth/me');
}
