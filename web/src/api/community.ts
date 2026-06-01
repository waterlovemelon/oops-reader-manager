import { request } from './http';
import { ListResponse } from './catalog';

export interface Thread {
  id: string;
  board_id: string;
  title: string;
  author_id: string;
  status: string;
  created_at: string;
  comment_count: number;
}

export interface Comment {
  id: string;
  thread_id: string;
  author_id: string;
  body: string;
  status: string;
  created_at: string;
}

export function listThreads(q?: string, page?: number, pageSize?: number) {
  const params = new URLSearchParams();
  if (q) params.set('q', q);
  if (page) params.set('page', String(page));
  if (pageSize) params.set('page_size', String(pageSize));
  return request<ListResponse<Thread>>(`/admin/community/threads?${params}`);
}

export function updateThreadStatus(id: string, status: string) {
  return request<{ data: { id: string; status: string } }>(`/admin/community/threads/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
}

export function listComments(threadId?: string, page?: number, pageSize?: number) {
  const params = new URLSearchParams();
  if (threadId) params.set('thread_id', threadId);
  if (page) params.set('page', String(page));
  if (pageSize) params.set('page_size', String(pageSize));
  return request<ListResponse<Comment>>(`/admin/community/comments?${params}`);
}

export function updateCommentStatus(id: string, status: string) {
  return request<{ data: { id: string; status: string } }>(`/admin/community/comments/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
}
