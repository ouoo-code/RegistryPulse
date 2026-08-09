import { api, type AdminRole, type AdminSettings, type AdminTask, type AdminUser, type Category, type CategoryInput, type CredentialProfile, type CredentialProfileInput, type History, type NotificationChannel, type NotificationRule, type ProbeNode, type ProbeTestInput, type ProbeTestResult, type Source, type TestImage, type TestImageQuery, type TotpSettings } from './api'

export function adminHeaders(token: string): HeadersInit {
  return { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) }
}

export function adminRequest<T>(token: string, path: string, options: RequestInit = {}) {
  return api<T>(path, { ...options, headers: { ...adminHeaders(token), ...(options.headers || {}) } })
}

export const adminApi = {
  sources: (token: string) => adminRequest<Source[]>(token, '/admin/sources'),
  testSource: (token: string, input: ProbeTestInput) => adminRequest<ProbeTestResult>(token, '/admin/sources/test', { method: 'POST', body: JSON.stringify(input) }),
  categories: (token: string) => adminRequest<Category[]>(token, '/admin/categories'),
  tasks: (token: string) => adminRequest<AdminTask[]>(token, '/admin/tasks?limit=100'),
  clearTasks: (token: string) => adminRequest<{ deleted: number }>(token, '/admin/tasks', { method: 'DELETE' }),
  results: (token: string) => adminRequest<History[]>(token, '/admin/results'),
  nodes: (token: string) => adminRequest<ProbeNode[]>(token, '/admin/probes'),
  users: (token: string) => adminRequest<AdminUser[]>(token, '/admin/users'),
  roles: (token: string) => adminRequest<AdminRole[]>(token, '/admin/roles'),
  notifications: (token: string) => adminRequest<NotificationChannel[]>(token, '/admin/notifications'),
  notificationRules: (token: string) => adminRequest<NotificationRule[]>(token, '/admin/notification-rules'),
  credentialProfiles: (token: string) => adminRequest<CredentialProfile[]>(token, '/admin/credential-profiles'),
  testImages: (token: string, query: TestImageQuery = {}) => {
    const params = new URLSearchParams()
    if (query.category_id) params.set('category_id', query.category_id)
    if (query.probe_mode) params.set('probe_mode', query.probe_mode)
    const suffix = params.toString()
    return adminRequest<TestImage[]>(token, `/admin/test-images${suffix ? `?${suffix}` : ''}`)
  },
  settings: (token: string) => adminRequest<AdminSettings>(token, '/admin/settings'),
  totp: (token: string) => adminRequest<TotpSettings>(token, '/admin/totp'),
  createCategory: (token: string, input: CategoryInput) => adminRequest<Category>(token, '/admin/categories', { method: 'POST', body: JSON.stringify(input) }),
  updateCategory: (token: string, id: string, input: CategoryInput) => adminRequest<Category>(token, `/admin/categories/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteCategory: (token: string, id: string) => adminRequest<void>(token, `/admin/categories/${id}`, { method: 'DELETE' }),
  createNotification: (token: string, input: Omit<NotificationChannel, 'id'>) => adminRequest<NotificationChannel>(token, '/admin/notifications', { method: 'POST', body: JSON.stringify(input) }),
  updateNotification: (token: string, id: string, input: Omit<NotificationChannel, 'id'>) => adminRequest<NotificationChannel>(token, `/admin/notifications/${id}`, { method: 'PUT', body: JSON.stringify({ ...input, id }) }),
  deleteNotification: (token: string, id: string) => adminRequest<void>(token, `/admin/notifications/${id}`, { method: 'DELETE' }),
  createNotificationRule: (token: string, input: Omit<NotificationRule, 'id'>) => adminRequest<NotificationRule>(token, '/admin/notification-rules', { method: 'POST', body: JSON.stringify(input) }),
  updateNotificationRule: (token: string, id: string, input: Omit<NotificationRule, 'id'>) => adminRequest<NotificationRule>(token, `/admin/notification-rules/${id}`, { method: 'PUT', body: JSON.stringify({ ...input, id }) }),
  deleteNotificationRule: (token: string, id: string) => adminRequest<void>(token, `/admin/notification-rules/${id}`, { method: 'DELETE' }),
  createCredentialProfile: (token: string, input: CredentialProfileInput) => adminRequest<CredentialProfile>(token, '/admin/credential-profiles', { method: 'POST', body: JSON.stringify(input) }),
  updateCredentialProfile: (token: string, id: string, input: CredentialProfileInput) => adminRequest<CredentialProfile>(token, `/admin/credential-profiles/${id}`, { method: 'PUT', body: JSON.stringify({ ...input, id }) }),
  deleteCredentialProfile: (token: string, id: string) => adminRequest<void>(token, `/admin/credential-profiles/${id}`, { method: 'DELETE' }),
}

export async function exportSources(token: string, format: 'json' | 'csv') {
  const response = await fetch(`/api/v1/admin/sources/export?format=${format}`, { headers: adminHeaders(token) })
  if (!response.ok) throw new Error(`API ${response.status}`)
  return response.blob()
}

export async function importSources(token: string, content: string, isCSV: boolean) {
  const response = await fetch('/api/v1/admin/sources/import', { method: 'POST', headers: { ...adminHeaders(token), 'Content-Type': isCSV ? 'text/csv' : 'application/json' }, body: content })
  if (!response.ok) throw new Error(`API ${response.status}`)
}
