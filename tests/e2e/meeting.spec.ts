import { expect, test } from '@playwright/test'

test('两名用户可以创建并加入同一会议', async ({ browser }) => {
  const firstContext = await browser.newContext()
  const secondContext = await browser.newContext()
  const firstPage = await firstContext.newPage()
  const secondPage = await secondContext.newPage()

  try {
    await firstPage.goto('/')
    await firstPage.getByRole('button', { name: 'New Meeting' }).click()
    await expect(firstPage).toHaveURL(/\/\d{3}-\d{3}-\d{3}$/)

    const meetingURL = firstPage.url()
    await firstPage.getByRole('button', { name: 'Join' }).click()
    await expect(firstPage.getByTestId('meeting-layout')).toBeVisible({ timeout: 15_000 })

    await secondPage.goto(meetingURL)
    await secondPage.getByRole('button', { name: 'Join' }).click()
    await secondPage.getByRole('button', { name: 'Join' }).click()
    await expect(secondPage.getByTestId('meeting-layout')).toBeVisible({ timeout: 15_000 })

    await expect(firstPage.getByTestId('meeting-layout')).toBeVisible()
  } finally {
    await secondContext.close()
    await firstContext.close()
  }
})

test('首页可以打开已有会议入口', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('button', { name: 'New Meeting' })).toBeVisible()
  await expect(page.getByPlaceholder('Enter Meeting id')).toBeVisible()
})
