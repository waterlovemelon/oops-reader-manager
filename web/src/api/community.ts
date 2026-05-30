import { request } from './http';

export interface Thread {
  id: string;
  board_id: string;
  title: string;
  author_id: string;
  status: string;
  created_at: string;
  comment_count: number;
}

export function listThreads(q?: string, page?: number) {
  const params = new URLSearchParams();
  if (q) params.set('q', q);
  if (page) params.set('page', String(page));
  return request<{ data: Thread[]; pagination: { page: number; page_size: number; total: number } }>(`/admin/community/threads?${params}`);
}
