import { expect, test } from '@playwright/test'

test('Vue 入口可以创建并进入会议', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: '新建会议' }).click()
  await expect(page.getByRole('heading', { name: '准备加入' })).toBeVisible()
  await page.getByRole('button', { name: '加入会议' }).click()
  await expect(page.getByRole('button', { name: '离开会议' })).toBeVisible({ timeout: 15_000 })
})
