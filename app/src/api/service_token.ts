import { http } from '@uozi-admin/request'

export type ServiceTokenScope = 'api:read' | 'api:write' | 'mcp:read' | 'mcp:write'

export interface ServiceToken {
  id: string
  name: string
  scopes: ServiceTokenScope[]
  expires_at?: string
  revoked_at?: string
  creator_id: number
  last_used_at?: string
  created_at: string
  token?: string
}

export interface CreateServiceTokenRequest {
  name: string
  scopes: ServiceTokenScope[]
  expires_at?: string
}

const serviceToken = {
  list(): Promise<ServiceToken[]> {
    return http.get('/service_tokens')
  },
  create(data: CreateServiceTokenRequest): Promise<ServiceToken> {
    return http.post('/service_tokens', data)
  },
  rotate(id: string): Promise<ServiceToken> {
    return http.post(`/service_tokens/${encodeURIComponent(id)}/rotate`)
  },
  revoke(id: string): Promise<void> {
    return http.delete(`/service_tokens/${encodeURIComponent(id)}`)
  },
}

export default serviceToken
