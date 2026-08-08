import { expect, test } from '@playwright/test'

test('public status browsing flow', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle(/Container Registry Monitor/)
  await expect(page.getByText('镜像源状态')).toBeVisible()
  await page.locator('article.source a').first().click()
  await expect(page.getByText(/响应趋势|Response trend/)).toBeVisible()

  await page.goto('/status/dockerhub')
  await expect(page.getByText(/Docker Hub|dockerhub/).first()).toBeVisible()

  await page.goto('/configure')
  await expect(page.getByText(/配置生成器|Config generator/)).toBeVisible()
  await expect(page.locator('pre')).toBeVisible()

  await page.goto('/tutorial')
  await expect(page.getByText(/Docker Linux/)).toBeVisible()
  await page.goto('/about')
  await expect(page.getByText(/Container Registry Monitor/).first()).toBeVisible()
})

test('public APIs and route fallbacks are reachable', async ({ page, request }) => {
  for (const path of ['/api/v1/health', '/api/v1/public/summary', '/api/v1/public/categories', '/api/v1/public/sources']) {
    const response = await request.get(path)
    expect(response.ok(), path).toBeTruthy()
    expect((await response.json()).success, path).toBe(true)
  }
  for (const path of ['/', '/status/ghcr', '/status/quay', '/status/mcr', '/status/k8s', '/status/gcr', '/status/elastic', '/status/nvcr']) {
    const response = await page.goto(path)
    expect(response?.ok(), path).toBeTruthy()
  }
})

test('public filters, theme, mobile menu, and configuration copy work', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await page.getByLabel('Search registries').fill('registry')
  await page.getByLabel('Filter tags').fill('official')
  await page.getByRole('button', { name: /Open menu/ }).click()
  await page.getByRole('button', { name: /Dark mode|Light mode|深色|浅色/ }).click()
  await page.goto('/configure')
  await page.locator('select').selectOption('1panel')
  await expect(page.locator('pre')).toContainText('registry-mirrors')
  await page.getByRole('button', { name: /复制|Copy/ }).click()
  await expect(page.locator('.generator-actions button')).toHaveText(/已复制|Copied/)
})
