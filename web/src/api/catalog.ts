const API_BASE_URL = import.meta.env.VITE_MANAGER_API_BASE_URL ?? '';

export interface CatalogBook {
  id: string;
  title: string;
  author: string;
  format: string;
  filename: string;
  file_size: number;
  chapter_count: number;
  status: string;
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
  const body = await response.json();
  if (!response.ok) {
    throw new Error(body.error ?? '上传失败');
  }
  return body.data as CatalogBook;
}
