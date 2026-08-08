export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api/v1${path}`, options)
  const body = await response.json().catch(() => null)
  if (!response.ok) throw new Error(body?.error?.message || `API ${response.status}`)
  if (body?.success === false) throw new Error(body.error?.message || 'API error')
  return body.data as T
}
export type SourceStatus = 'online' | 'degraded' | 'offline' | 'maintenance' | string
export type Source = { id: string; name: string; base_url: string; display_url?: string; registry_host?: string; description?: string; category_id: string; provider?: string; country?: string; region?: string; operator?: string; tags: string[]; is_official?: boolean; is_cloudflare?: boolean; is_recommended?: boolean; enabled?: boolean; priority?: number; sort_order?: number; status: SourceStatus; response_ms: number; last_checked: string; created_at?: string; updated_at?: string; maintenance?: boolean; probe_mode?: string; test_repository?: string; test_tag?: string; test_digest?: string; test_image_id?: string; test_image_reference?: string; test_image_max_bytes?: number; request_timeout_seconds?: number; download_test_bytes?: number; error?: string }
export type Category = { id: string; slug: string; name: string; description: string; icon?: string; official_url?: string; default_test_repository?: string; default_manifest_path?: string; auth_type?: string; enabled?: boolean; sort_order?: number; source_count?: number; created_at?: string; updated_at?: string }
export type CategoryInput = Pick<Category, 'id' | 'slug' | 'name'> & Partial<Omit<Category, 'id' | 'slug' | 'name'>>
export type AdminTask = { id: string; source_id: string; probe_node_id?: string; task_type?: string; status: string; created_at?: string; started_at?: string; finished_at?: string; error?: string }
export type ProbeNode = { id: string; name: string; region?: string; version?: string; status?: string; last_seen_at?: string }
export type AdminUser = { id: string; username: string; roles?: string[] | string; active?: boolean; created_at?: string }
export type AdminRole = { name: string; permissions?: string[] | string }
export type NotificationChannel = { id: string; name: string; type: string; enabled?: boolean; config?: Record<string, unknown>; created_at?: string; updated_at?: string }
export type NotificationRule = { id: string; event_type: string; channel_id: string; cooldown_seconds: number; aggregation_seconds: number; template?: string; enabled?: boolean }
export type TestImage = { id: string; reference: string; enabled: boolean; max_bytes: number; is_default: boolean; created_at?: string; updated_at?: string }
export type AdminSettings = Record<string, unknown>
export type TotpSettings = { enabled: boolean; secret?: string; otpauth_uri?: string }
export type AggregatePoint = { bucket: string; samples: number; online_samples: number; avg_duration_ms: number }
export type SourceAggregates = { hourly: AggregatePoint[]; daily: AggregatePoint[] }
export type History = { status: string; response_ms: number; checked_at?: string; created_at?: string; error?: string; error_stage?: string; dns_duration_ms?: number; tcp_duration_ms?: number; tls_duration_ms?: number; registry_duration_ms?: number; manifest_duration_ms?: number; blob_duration_ms?: number; blob_bytes?: number; blob_speed_bps?: number; blob_ttfb_ms?: number; dns_success?: boolean; tcp_success?: boolean; tls_success?: boolean; registry_api_success?: boolean; manifest_success?: boolean; blob_success?: boolean; certificate_not_before?: string; certificate_not_after?: string; registry_api_version?: string; manifest_size?: number; blob_range_supported?: boolean; dns_error?: string; tcp_error?: string; tls_error?: string; registry_api_error?: string; manifest_error?: string; blob_error?: string }
export type SourceInput = { name: string; base_url: string; display_url?: string; description?: string; category_id: string; provider?: string; country?: string; region?: string; operator?: string; tags?: string[]; is_official?: boolean; is_cloudflare?: boolean; is_recommended?: boolean; enabled?: boolean; priority?: number; sort_order?: number; maintenance?: boolean; probe_mode?: string; test_repository?: string; test_tag?: string; test_digest?: string; test_image_id?: string; request_timeout_seconds?: number; download_test_bytes?: number }
