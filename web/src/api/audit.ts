import { request } from './http';
import { ListResponse } from './catalog';

export interface AuditEntry {
  id: number;
  admin_username: string;
  action: string;
  resource_type: string;
  resource_id: string;
  before_json: string;
  after_json: string;
  ip_address: string;
  user_agent: string;
  created_at: string;
}

export interface AuditQuery {
  admin?: string;
  resource_type?: string;
  start?: string;
  end?: string;
  page?: number;
  page_size?: number;
}

export function listAuditLogs(params: AuditQuery = {}) {
  const searchParams = new URLSearchParams();
  if (params.admin) searchParams.set('admin', params.admin);
  if (params.resource_type) searchParams.set('resource_type', params.resource_type);
  if (params.start) searchParams.set('start', params.start);
  if (params.end) searchParams.set('end', params.end);
  if (params.page) searchParams.set('page', String(params.page));
  if (params.page_size) searchParams.set('page_size', String(params.page_size));
  return request<ListResponse<AuditEntry>>(`/admin/audit/logs?${searchParams}`);
}
