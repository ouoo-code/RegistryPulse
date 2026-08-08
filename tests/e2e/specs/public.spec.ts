import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('locale', 'en')
    localStorage.setItem('theme', 'light')
  })
})

test('public status browsing flow', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveTitle(/Registry Pulse - Container Registry Monitor/)
  await expect(page.getByText('Registry status').first()).toBeVisible()
  await page.locator('.registry-row a').first().click()
  await expect(page.getByText('Response trend')).toBeVisible()

  await page.goto('/status/dockerhub')
  await expect(page.getByText(/Docker Hub|dockerhub/).first()).toBeVisible()

  await page.goto('/configure')
  await expect(page.getByText('Config generator').first()).toBeVisible()
  await expect(page.locator('pre')).toBeVisible()

  await page.goto('/tutorial')
  await expect(page.getByText('Configure container registries')).toBeVisible()
  await page.goto('/about')
  await expect(page.getByText('Registry Pulse').first()).toBeVisible()
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
  await page.getByLabel('Search name, domain or provider').fill('registry')
  await page.getByPlaceholder('Filter by tag').fill('official')
  await page.getByRole('button', { name: 'Open menu' }).click()
  await page.getByRole('button', { name: /Dark mode|Light mode/ }).click()
  await page.goto('/configure')
  await page.locator('select').first().selectOption({ index: 0 })
  await expect(page.locator('pre')).toContainText('registry-mirrors')
  await page.getByRole('button', { name: 'Copy' }).click()
  await expect(page.locator('.generator-output button')).toHaveText('Copied')
})
