import { request } from './http';
import { ListResponse } from './catalog';

export interface User {
  id: string;
  email: string;
  display_name: string;
  status: string;
  created_at: string;
}

export function listUsers(q?: string, page?: number, pageSize?: number) {
  const params = new URLSearchParams();
  if (q) params.set('q', q);
  if (page) params.set('page', String(page));
  if (pageSize) params.set('page_size', String(pageSize));
  return request<ListResponse<User>>(`/admin/users?${params}`);
}

export function updateUserStatus(id: string, status: string) {
  return request<{ data: { id: string; status: string } }>(`/admin/users/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
}
