import { request } from './http';
import type { CatalogBook, ListResponse } from './catalog';

export interface BookRecommendation {
  id: string;
  book_key: string;
  comment: string;
  status: string;
  publish_state: string;
  scheduled_publish_at: string | null;
  created_by: string;
  updated_by: string;
  created_at: string;
  updated_at: string;
  book: CatalogBook | null;
}

export interface CreateRecommendationInput {
  book_key: string;
  comment?: string;
  publish_state?: string;
  scheduled_publish_at?: string;
}

export interface UpdateRecommendationInput {
  book_key?: string;
  comment?: string;
  publish_state?: string;
  scheduled_publish_at?: string | null;
}

export function listRecommendations(q?: string, status?: string, page?: number, pageSize?: number) {
  const params = new URLSearchParams();
  if (q) params.set('q', q);
  if (status) params.set('status', status);
  if (page) params.set('page', String(page));
  if (pageSize) params.set('page_size', String(pageSize));
  return request<ListResponse<BookRecommendation>>(`/admin/recommendations/books?${params}`);
}

export function getRecommendation(id: string) {
  return request<{ data: BookRecommendation }>(`/admin/recommendations/books/${encodeURIComponent(id)}`);
}

export function createRecommendation(input: CreateRecommendationInput) {
  return request<{ data: BookRecommendation }>('/admin/recommendations/books', {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export function updateRecommendation(id: string, input: UpdateRecommendationInput) {
  return request<{ data: BookRecommendation }>(`/admin/recommendations/books/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  });
}

export function deleteRecommendation(id: string) {
  return request<{ data: { id: string; status: string } }>(`/admin/recommendations/books/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}
