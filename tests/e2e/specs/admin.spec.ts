import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('locale', 'en')
    localStorage.setItem('theme', 'light')
  })
})

async function signIn(page: import('@playwright/test').Page) {
  await page.goto('/admin')
  await page.getByLabel('Username').fill(process.env.E2E_ADMIN_USERNAME || 'admin')
  await page.getByLabel('Password').fill(process.env.E2E_ADMIN_PASSWORD as string)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByText('Registry Source Admin')).toBeVisible()
}

test('admin token can load the source management page', async ({ page }) => {
  test.skip(!process.env.E2E_ADMIN_PASSWORD, 'Set E2E_ADMIN_PASSWORD to run the authenticated admin flow.')
  await signIn(page)
  await expect(page.getByRole('button', { name: 'Sources', exact: true })).toBeVisible()
})

test('test image editor keeps its structured layout', async ({ page }) => {
  await page.addInitScript(() => {
    sessionStorage.setItem('admin-token', 'layout-test-token')
  })
  await page.route('**/api/v1/admin/sources', route => route.fulfill({ json: { success: true, data: [] } }))
  await page.route('**/api/v1/admin/categories', route => route.fulfill({
    json: {
      success: true,
      data: [
        { id: 'dockerhub', slug: 'DockerHub', name: 'Docker Hub', description: '', enabled: true },
        { id: 'ghcr', slug: 'GHCR', name: 'GitHub Container Registry', description: '', enabled: true },
        { id: 'quay', slug: 'Quay', name: 'Quay', description: '', enabled: true },
      ],
    },
  }))
  await page.route('**/api/v1/admin/test-images**', route => route.fulfill({ json: { success: true, data: [] } }))
  await page.goto('/admin')
  await page.getByRole('button', { name: 'Test images', exact: true }).click()
  await page.locator('.admin-resource-section').getByRole('button', { name: 'Add', exact: true }).click()

  const form = page.locator('.test-image-editor-form')
  const scopeFields = form.locator('.test-image-scope-field')
  const firstCheckbox = form.locator('.checkbox-list-item input[type="checkbox"]').first()
  const footer = form.locator('.test-image-editor-footer')
  await expect(form).toHaveCSS('display', 'flex')
  await expect(scopeFields).toHaveCount(2)
  await expect(firstCheckbox).toHaveCSS('width', '16px')

  const categoryBox = await scopeFields.nth(0).boundingBox()
  const modeBox = await scopeFields.nth(1).boundingBox()
  const footerBox = await footer.boundingBox()
  expect(categoryBox).not.toBeNull()
  expect(modeBox).not.toBeNull()
  expect(footerBox).not.toBeNull()
  expect(categoryBox!.width).toBeGreaterThan(300)
  expect(modeBox!.width).toBeGreaterThan(300)
  expect(Math.abs(categoryBox!.y - modeBox!.y)).toBeLessThan(4)
  expect(footerBox!.y).toBeGreaterThan(categoryBox!.y + categoryBox!.height)
})

test('administrator can add, probe, inspect, and remove a source', async ({ page }) => {
  test.skip(!process.env.E2E_ADMIN_PASSWORD, 'Set E2E_ADMIN_PASSWORD to run the authenticated admin flow.')
  const name = `E2E Registry ${Date.now()}`
  page.on('dialog', dialog => dialog.accept())
  await signIn(page)
  await page.getByRole('button', { name: 'Add', exact: true }).last().click()
  const editor = page.locator('.admin-editor-form')
  await editor.getByLabel('Name').fill(name)
  await editor.getByLabel('URL').fill('https://registry-1.docker.io')
  await editor.getByLabel('Category').selectOption('dockerhub')
  await editor.getByRole('button', { name: 'Save', exact: true }).click()
  const row = page.getByRole('row').filter({ hasText: name })
  await expect(row).toBeVisible()
  await row.getByRole('button', { name: 'Probe', exact: true }).click()
  await expect(page.getByText('Probe task queued')).toBeVisible()
  await page.getByRole('button', { name: 'Tasks', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Probe tasks', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Sources', exact: true }).click()
  await row.getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(row).toHaveCount(0)
})
