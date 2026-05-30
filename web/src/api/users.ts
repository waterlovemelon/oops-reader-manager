import { request } from './http';

export interface User {
  id: string;
  email: string;
  display_name: string;
  status: string;
  created_at: string;
}

export function listUsers(q?: string, page?: number) {
  const params = new URLSearchParams();
  if (q) params.set('q', q);
  if (page) params.set('page', String(page));
  return request<{ data: User[]; pagination: { page: number; page_size: number; total: number } }>(`/admin/users?${params}`);
}
