import { request } from './http';

const API_BASE_URL = import.meta.env.VITE_MANAGER_API_BASE_URL ?? '';

export interface CatalogBook {
  book_key: string;
  title: string;
  author: string;
  description: string;
  format: string;
  filename: string;
  storage_path: string;
  cover_storage_path: string;
  file_size: number;
  content_sha1: string;
  language: string;
  chapter_count: number;
  word_count?: number;
  status: string;
  source: string;
  uploaded_at: string;
  published_at: string;
  updated_by: string;
}

export interface ListResponse<T> {
  data: T[];
  pagination: { page: number; page_size: number; total: number };
}

export async function uploadBook(file: File): Promise<CatalogBook> {
  const token = localStorage.getItem('manager_access_token');
  const form = new FormData();
  form.append('file', file);
  const response = await fetch(`${API_BASE_URL}/admin/catalog/books/upload`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: form,
  });
  const body = await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(body?.error ?? `上传失败 (${response.status})`);
  }
  if (!body?.data) {
    throw new Error('服务器返回数据格式错误');
  }
  return body.data as CatalogBook;
}

export function listBooks(q?: string, status?: string, page?: number, pageSize?: number) {
  const params = new URLSearchParams();
  if (q) params.set('q', q);
  if (status) params.set('status', status);
  if (page) params.set('page', String(page));
  if (pageSize) params.set('page_size', String(pageSize));
  return request<ListResponse<CatalogBook>>(`/admin/catalog/books?${params}`);
}

export function getBook(bookKey: string) {
  return request<{ data: CatalogBook }>(`/admin/catalog/books/${encodeURIComponent(bookKey)}`);
}

export function updateBook(bookKey: string, data: { title?: string; author?: string; description?: string; language?: string }) {
  return request<{ data: CatalogBook }>(`/admin/catalog/books/${encodeURIComponent(bookKey)}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export function updateBookStatus(bookKey: string, status: string) {
  return request<{ data: { book_key: string; status: string } }>(`/admin/catalog/books/${encodeURIComponent(bookKey)}/status`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  });
}

// --- Async Import Jobs ---

export interface ImportJob {
  job_id: string;
  admin_username: string;
  original_filename: string;
  format: string;
  content_sha1: string;
  file_size: number;
  status: 'queued' | 'processing' | 'succeeded' | 'failed' | 'canceled';
  stage: string;
  progress_percent?: number;
  attempt_count: number;
  max_attempts: number;
  book_key?: string;
  error_code?: string;
  error_message?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  updated_at: string;
}

export interface CreateImportJobResponse {
  job_id: string;
  status: string;
  stage: string;
}

export async function createImportJob(file: File): Promise<CreateImportJobResponse> {
  const token = localStorage.getItem('manager_access_token');
  const form = new FormData();
  form.append('file', file);
  const response = await fetch(`${API_BASE_URL}/admin/catalog/import-jobs`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: form,
  });
  const body = await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(body?.error ?? `上传失败 (${response.status})`);
  }
  if (!body?.data) {
    throw new Error('服务器返回数据格式错误');
  }
  return body.data as CreateImportJobResponse;
}

export function getImportJob(jobID: string) {
  return request<{ data: ImportJob }>(`/admin/catalog/import-jobs/${encodeURIComponent(jobID)}`);
}

export function listImportJobs(status?: string, page?: number, pageSize?: number) {
  const params = new URLSearchParams();
  if (status) params.set('status', status);
  if (page) params.set('page', String(page));
  if (pageSize) params.set('page_size', String(pageSize));
  return request<ListResponse<ImportJob>>(`/admin/catalog/import-jobs?${params}`);
}

export function retryImportJob(jobID: string) {
  return request<{ data: { job_id: string; status: string } }>(`/admin/catalog/import-jobs/${encodeURIComponent(jobID)}/retry`, {
    method: 'POST',
  });
}

export function cancelImportJob(jobID: string) {
  return request<{ data: { job_id: string; status: string } }>(`/admin/catalog/import-jobs/${encodeURIComponent(jobID)}/cancel`, {
    method: 'POST',
  });
}

export async function fetchBookCover(bookKey: string): Promise<string | null> {
  const token = localStorage.getItem('manager_access_token');
  const response = await fetch(
    `${API_BASE_URL}/admin/catalog/books/${encodeURIComponent(bookKey)}/cover`,
    { headers: token ? { Authorization: `Bearer ${token}` } : {} }
  );
  if (!response.ok) return null;
  const blob = await response.blob();
  return URL.createObjectURL(blob);
}
