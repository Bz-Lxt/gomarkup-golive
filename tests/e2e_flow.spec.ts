import { test, expect } from '@playwright/test'

const BASE = process.env.GOLIVE_URL || 'http://127.0.0.1:19443'

test.describe('GoLive console', () => {
  test('health page shell and capability path', async ({ page }) => {
    await page.goto(BASE)
    const hasWT = await page.evaluate(() => typeof (window as unknown as { WebTransport?: unknown }).WebTransport === 'function')
    if (!hasWT) {
      await expect(page.getByText('当前浏览器没有 WebTransport')).toBeVisible()
      return
    }
    await expect(page.getByText('GoLive 弱网对抗台')).toBeVisible()
    await expect(page.getByRole('button', { name: '连接' })).toBeVisible()
    await page.getByRole('button', { name: '连接' }).click()
    await expect(page.getByText(/会话/)).toBeVisible({ timeout: 15000 })
    await page.getByRole('button', { name: '30%' }).click()
    await expect(page.getByText('损伤配置已热切换')).toBeVisible({ timeout: 8000 })
  })
})
