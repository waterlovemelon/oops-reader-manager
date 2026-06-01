import { request } from './http';

export interface Entitlement {
  id: number;
  user_id: number;
  entitlement_key: string;
  status: string;
  source: string;
  starts_at: string;
  expires_at: string | null;
  created_at: string;
}

export function listEntitlements(userId: string) {
  return request<{ data: Entitlement[] }>(`/admin/users/${userId}/entitlements`);
}

export function createEntitlement(userId: string, data: {
  entitlement_key: string;
  source?: string;
  starts_at: string;
  expires_at?: string;
}) {
  return request<{ data: { id: number } }>(`/admin/users/${userId}/entitlements`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export function revokeEntitlement(id: number) {
  return request<{ data: { id: number; status: string } }>(`/admin/entitlements/${id}/revoke`, {
    method: 'POST',
  });
}

export function extendEntitlement(id: number, expiresAt: string) {
  return request<{ data: { id: number; expires_at: string } }>(`/admin/entitlements/${id}/extend`, {
    method: 'POST',
    body: JSON.stringify({ expires_at: expiresAt }),
  });
}
