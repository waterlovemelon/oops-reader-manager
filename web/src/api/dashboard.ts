import { request } from './http';

export interface DashboardSummary {
  total_users: number;
  total_books: number;
  total_threads: number;
  pending_review: number;
}

export function getSummary() {
  return request<{ data: DashboardSummary }>('/admin/dashboard/summary');
}
