export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api/v1${path}`, options)
  const body = await response.json().catch(() => null)
  if (!response.ok) throw new Error(body?.error?.message || `API ${response.status}`)
  if (body?.success === false) throw new Error(body.error?.message || 'API error')
  return body.data as T
}
export type SourceStatus = 'online' | 'degraded' | 'offline' | 'maintenance' | string
export type Source = { id: string; name: string; base_url: string; display_url?: string; registry_host?: string; description?: string; category_id: string; provider?: string; country?: string; region?: string; operator?: string; tags: string[]; is_official?: boolean; is_cloudflare?: boolean; is_recommended?: boolean; enabled?: boolean; priority?: number; sort_order?: number; status: SourceStatus; response_ms: number; last_checked: string; created_at?: string; updated_at?: string; maintenance?: boolean; probe_config_custom?: boolean; probe_mode?: string; test_repository?: string; test_tag?: string; test_digest?: string; test_image_id?: string; test_image_reference?: string; test_image_max_bytes?: number; request_timeout_seconds?: number; download_test_bytes?: number; error?: string }
export type Category = { id: string; slug: string; name: string; description: string; icon?: string; official_url?: string; default_test_repository?: string; default_test_tag?: string; default_test_image_id?: string; default_probe_mode?: string; default_timeout_seconds?: number; default_manifest_path?: string; auth_type?: string; enabled?: boolean; sort_order?: number; source_count?: number; available_test_image_ids?: string[]; available_test_images?: TestImage[]; test_images?: TestImage[]; created_at?: string; updated_at?: string }
export type CategoryInput = Pick<Category, 'id' | 'slug' | 'name'> & Partial<Omit<Category, 'id' | 'slug' | 'name'>>
export type AdminTask = { id: string; source_id: string; probe_node_id?: string; task_type?: string; status: string; created_at?: string; started_at?: string; finished_at?: string; error?: string }
export type ProbeNode = { id: string; name: string; region?: string; version?: string; status?: string; last_seen_at?: string }
export type AdminUser = { id: string; username: string; roles?: string[] | string; active?: boolean; totp_enabled?: boolean; created_at?: string }
export type AdminRole = { name: string; permissions?: string[] | string; user_count?: number }
export type AdminUserInput = { username: string; password?: string; role?: string; active?: boolean }
export type AdminRoleInput = { name: string; permissions: string[] }
export type NotificationChannel = { id: string; name: string; type: string; enabled?: boolean; config?: Record<string, unknown>; created_at?: string; updated_at?: string }
export type NotificationRule = { id: string; event_type: string; channel_id: string; cooldown_seconds: number; aggregation_seconds: number; template?: string; enabled?: boolean }
export type TestImageCategoryRef = string | { id?: string; slug?: string }
export type TestImageAuth = { strategy?: string; type?: string; configured?: boolean; secret_configured?: boolean; has_secret?: boolean; secret_masked?: string }
export type TestImage = {
  id: string
  reference: string
  enabled: boolean
  max_bytes: number
  is_default: boolean
  created_at?: string
  updated_at?: string
  applicable_category_ids?: string[]
  applicable_categories?: TestImageCategoryRef[]
  category_ids?: string[]
  applicable_probe_modes?: string[]
  probe_modes?: string[]
  auth_strategy?: string
  auth_type?: string
  auth_configured?: boolean
  has_secret?: boolean
  secret_masked?: string
  auth?: TestImageAuth
}
export type TestImageQuery = { category_id?: string; probe_mode?: string }
export type TestImageInput = {
  id?: string
  reference: string
  enabled: boolean
  max_bytes: number
  is_default?: boolean
  category_ids?: string[]
  probe_modes?: string[]
  applicable_category_ids?: string[]
  applicable_probe_modes?: string[]
  auth_strategy?: string
}
export type CredentialProfile = {
  id: string
  name: string
  auth_type: 'basic' | 'bearer' | 'token' | string
  username?: string
  source_id?: string
  registry_host?: string
  category_id?: string
  enabled: boolean
  has_secret?: boolean
  secret_masked?: string
  created_at?: string
  updated_at?: string
}
export type CredentialProfileInput = {
  id?: string
  name: string
  auth_type: 'basic' | 'bearer' | 'token' | string
  username?: string
  secret?: string
  clear_secret?: boolean
  source_id?: string
  registry_host?: string
  category_id?: string
  enabled?: boolean
}
export type AdminSettings = Record<string, unknown>
export type ProxyConfig = {
  enabled: boolean
  transport_mode: 'forward' | 'redirect'
  category_id: string
  route_max_age_minutes: number
  failure_cooldown_seconds: number
  max_concurrent: number
  max_range_mb: number
  max_manifest_mb: number
  updated_at?: string
}
export type ProxyStatus = {
  running: boolean
  enabled: boolean
  transport_mode?: 'forward' | 'redirect'
  ready: boolean
  actual_port: number
  configured_port: number
  category_id: string
  candidate_count: number
  last_error?: string
  started_at?: string
  last_seen_at?: string
}
export type AdminProxy = {
  config: ProxyConfig
  status: ProxyStatus
  status_available: boolean
  control_snapshot_published: boolean
}
export type ProxyMetrics = {
  requests: number
  successes: number
  upstream_failures: number
  retries: number
  redirects: number
  active_requests: number
  bytes_forwarded: number
  responses_1xx: number
  responses_2xx: number
  responses_3xx: number
  responses_4xx: number
  responses_5xx: number
  average_duration_seconds: number
  collected_at?: string
}
export type TotpSettings = { enabled: boolean; configured?: boolean; secret?: string; otpauth_uri?: string }
export type AggregatePoint = { bucket: string; samples: number; online_samples: number; avg_duration_ms: number }
export type SourceAggregates = { hourly: AggregatePoint[]; daily: AggregatePoint[] }
export type History = { id?: string; source_id: string; status: string; response_ms: number; checked_at?: string; created_at?: string; error?: string; error_stage?: string; dns_duration_ms?: number; tcp_duration_ms?: number; tls_duration_ms?: number; registry_duration_ms?: number; manifest_duration_ms?: number; blob_duration_ms?: number; blob_bytes?: number; blob_speed_bps?: number; blob_ttfb_ms?: number; dns_success?: boolean; tcp_success?: boolean; tls_success?: boolean; registry_api_success?: boolean; manifest_success?: boolean; blob_success?: boolean; certificate_not_before?: string; certificate_not_after?: string; registry_api_version?: string; manifest_size?: number; blob_range_supported?: boolean; dns_error?: string; tcp_error?: string; tls_error?: string; registry_api_error?: string; manifest_error?: string; blob_error?: string }
export type SourceInput = { name: string; base_url: string; display_url?: string; description?: string; category_id: string; provider?: string; country?: string; region?: string; operator?: string; tags?: string[]; is_official?: boolean; is_cloudflare?: boolean; is_recommended?: boolean; enabled?: boolean; priority?: number; sort_order?: number; maintenance?: boolean; probe_config_custom?: boolean; probe_mode?: string; test_repository?: string; test_tag?: string; test_digest?: string; test_image_id?: string; request_timeout_seconds?: number; download_test_bytes?: number }
export type ProbeTestInput = { base_url: string; probe_mode: string; request_timeout_seconds: number; test_repository?: string; test_tag?: string; test_image_reference?: string; download_test_bytes?: number }
export type ProbeTestResult = { status: string; error?: string; error_stage?: string; checked_at?: string; [key: string]: unknown }
