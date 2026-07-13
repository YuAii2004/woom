import { expect, test } from '@playwright/test'

test('两名用户可以创建并加入同一会议', async ({ browser }) => {
  const firstContext = await browser.newContext()
  const secondContext = await browser.newContext()
  const firstPage = await firstContext.newPage()
  const secondPage = await secondContext.newPage()

  try {
    await firstPage.goto('/')
    await firstPage.getByRole('button', { name: '新建会议' }).click()
    await expect(firstPage.getByRole('heading', { name: '准备加入' })).toBeVisible()
    await firstPage.getByRole('button', { name: '加入会议' }).click()
    await expect(firstPage.getByRole('button', { name: '离开会议' })).toBeVisible({ timeout: 15_000 })

    const meetingId = await firstPage.locator('#meeting-id').innerText()
    await secondPage.goto('/')
    await secondPage.getByPlaceholder('输入会议号').fill(meetingId)
    await secondPage.getByRole('button', { name: '加入' }).click()
    await expect(secondPage.getByRole('heading', { name: '准备加入' })).toBeVisible()
    await secondPage.getByRole('button', { name: '加入会议' }).click()
    await expect(secondPage.getByRole('button', { name: '离开会议' })).toBeVisible({ timeout: 15_000 })
    await expect(firstPage.getByRole('button', { name: '离开会议' })).toBeVisible()
  } finally {
    await secondContext.close()
    await firstContext.close()
  }
})

test('首页可以打开已有会议入口', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('button', { name: '新建会议' })).toBeVisible()
  await expect(page.getByPlaceholder('输入会议号')).toBeVisible()
})
