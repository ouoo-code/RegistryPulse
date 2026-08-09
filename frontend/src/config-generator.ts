function uniqueUrls(urls: string[]): string[] {
  return [...new Set(urls.map(url => url.trim()).filter(url => {
    try { const parsed = new URL(url); return parsed.protocol === 'http:' || parsed.protocol === 'https:' } catch { return false }
  }))].slice(0, 10)
}

export function renderDockerMirrors(urls: string[]): string { return JSON.stringify({ 'registry-mirrors': uniqueUrls(urls) }, null, 2) }
// 1Panel exposes Docker's daemon mirror settings in its container runtime UI.
// Keep the generated payload JSON-compatible so it can be pasted directly there.
export const renderOnePanelMirrors = renderDockerMirrors
const officialRegistries: Record<string, string> = {
  dockerhub: 'docker.io',
  ghcr: 'ghcr.io',
  quay: 'quay.io',
  mcr: 'mcr.microsoft.com',
  k8s: 'registry.k8s.io',
  gcr: 'gcr.io',
  elastic: 'docker.elastic.co',
  nvcr: 'nvcr.io',
}

function registryReference(url: string): string {
  const parsed = new URL(url)
  return parsed.host + parsed.pathname.replace(/\/$/, '')
}

export function renderPodmanMirrors(urls: string[], categorySlug = 'dockerhub'): string {
  const categoryKey = categorySlug.trim().toLowerCase()
  const official = officialRegistries[categoryKey] || categorySlug || 'docker.io'
  const mirrors = uniqueUrls(urls).map(url => '[[registry.mirror]]\nlocation = "' + registryReference(url) + '"\ninsecure = false').join('\n\n')
  const search = categoryKey === 'dockerhub' ? 'unqualified-search-registries = ["docker.io"]\n\n' : ''
  return search + '[[registry]]\nprefix = "' + official + '"\nlocation = "' + official + '"\ninsecure = false\n\n' + mirrors + '\n'
}
export function renderContainerdMirrors(urls: string[]): string { return uniqueUrls(urls).map(url => { const host = new URL(url).host; return `server = "https://${host}"\n\n[host."${url.replace(/\/$/, '')}"]\n  capabilities = ["pull", "resolve"]\n  skip_verify = false` }).join('\n\n') + '\n' }
export const renderNerdctlMirrors = renderContainerdMirrors

export function renderRegistryPullCommands(urls: string[], categorySlug: string): string {
  const official = officialRegistries[categorySlug.trim().toLowerCase()] || '<official-registry>'
  return uniqueUrls(urls).map(url => {
    const mirror = registryReference(url)
    return `# ${mirror}\ndocker pull ${mirror}/user/image:tag\ndocker tag ${mirror}/user/image:tag ${official}/user/image:tag`
  }).join('\n\n') + '\n'
}
