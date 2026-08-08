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

test('administrator can add, probe, inspect, and remove a source', async ({ page }) => {
  test.skip(!process.env.E2E_ADMIN_PASSWORD, 'Set E2E_ADMIN_PASSWORD to run the authenticated admin flow.')
  const name = `E2E Registry ${Date.now()}`
  page.on('dialog', dialog => dialog.accept())
  await signIn(page)
  await page.getByRole('button', { name: 'Add', exact: true }).last().click()
  await page.getByLabel('Name').fill(name)
  await page.getByLabel('URL').fill('https://registry-1.docker.io')
  await page.getByLabel('Category').last().selectOption('dockerhub')
  await page.getByRole('button', { name: 'Save', exact: true }).last().click()
  const row = page.getByRole('row').filter({ hasText: name })
  await expect(row).toBeVisible()
  await row.getByRole('button', { name: 'Probe', exact: true }).click()
  await expect(page.getByText('Probe task queued')).toBeVisible()
  await page.getByRole('button', { name: 'Tasks', exact: true }).click()
  await expect(page.getByText('Probe tasks')).toBeVisible()
  await page.getByRole('button', { name: 'Sources', exact: true }).click()
  await row.getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(row).toHaveCount(0)
})
