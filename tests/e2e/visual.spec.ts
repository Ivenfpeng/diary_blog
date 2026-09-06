import { expect, test, type Locator, type Page, type TestInfo } from '@playwright/test'
import { password, username } from './global-setup'

const visualArticle = {
  title: 'Responsive release notes',
  slug: 'responsive-release-notes',
  markdown: `# Responsive release notes

This article gives the release gate enough prose to exercise the reading layout.

## Editor and reader surfaces

Long prose stays inside the reading column at every supported viewport.

### Code stays contained

\`\`\`typescript
const aVeryLongCodeLine = 'release-gate-code-should-scroll-within-the-code-block-instead-of-expanding-the-page-viewport'
\`\`\`

The table of contents remains available without covering the article body.`,
}

const viewports = [
  { name: 'desktop', width: 1440, height: 1000 },
  { name: 'tablet', width: 1024, height: 768 },
  { name: 'mobile-wide', width: 390, height: 844 },
  { name: 'mobile-narrow', width: 360, height: 800 },
]

async function signIn(page: Page): Promise<void> {
  await page.goto('/admin/login')
  await page.getByLabel('Username').fill(username)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
}

async function seedPublishedArticle(page: Page): Promise<void> {
  await page.evaluate(async (article) => {
    const csrf = document.cookie.split('; ').find((cookie) => cookie.startsWith('diary_csrf='))?.slice('diary_csrf='.length)
    const request = async <T>(path: string, body: unknown): Promise<T> => {
      const response = await fetch(path, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': decodeURIComponent(csrf ?? '') },
        body: JSON.stringify(body),
      })
      if (!response.ok) throw new Error(`${path} returned ${response.status}`)
      return response.json() as Promise<T>
    }
    const category = await request<{ category: { id: number } }>('/api/admin/categories', { name: 'Visual checks', slug: 'visual-checks' })
    const created = await request<{ post: { id: number; revision: number } }>('/api/admin/posts', {
      slug: article.slug,
      title: article.title,
      summary: 'A layout-verification fixture.',
      content_md: article.markdown,
      category_id: category.category.id,
      tag_ids: [],
    })
    await request(`/api/admin/posts/${created.post.id}/publish`, { expected_revision: created.post.revision })
  }, visualArticle)
}

async function assertWithinViewport(page: Page, locator: Locator): Promise<void> {
  await expect(locator).toBeVisible()
  const box = await locator.boundingBox()
  expect(box, `${await locator.evaluate((element) => element.className || element.tagName)} has a layout box`).not.toBeNull()
  expect(box!.x).toBeGreaterThanOrEqual(-1)
  expect(box!.y).toBeGreaterThanOrEqual(-1)
  expect(box!.x + box!.width).toBeLessThanOrEqual(page.viewportSize()!.width + 1)
}

async function assertPageDoesNotOverflow(page: Page): Promise<void> {
  const dimensions = await page.evaluate(() => ({ clientWidth: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth }))
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth + 1)
}

async function screenshot(page: Page, info: TestInfo, name: string): Promise<void> {
  await page.screenshot({ path: info.outputPath(`${name}.png`), fullPage: true })
}

test('public and administration layouts fit required desktop and mobile viewports', async ({ page }, testInfo) => {
  await signIn(page)
  await seedPublishedArticle(page)

  for (const viewport of viewports) {
    await page.setViewportSize(viewport)

    await page.goto('/')
    await assertWithinViewport(page, page.locator('.site-header'))
    await assertWithinViewport(page, page.getByRole('heading', { name: 'Latest writing' }))
    if (viewport.width <= 800) await assertWithinViewport(page, page.locator('.mobile-navigation summary'))
    else await assertWithinViewport(page, page.locator('.header-nav'))
    await assertPageDoesNotOverflow(page)
    await screenshot(page, testInfo, `${viewport.name}-home`)

    await page.goto(`/posts/${visualArticle.slug}`)
    await assertWithinViewport(page, page.locator('.article-header h1'))
    await assertWithinViewport(page, page.locator('.prose'))
    await assertWithinViewport(page, page.locator('.prose pre'))
    if (viewport.width <= 800) await assertWithinViewport(page, page.locator('.mobile-table-of-contents'))
    else await assertWithinViewport(page, page.locator('.desktop-table-of-contents'))
    await assertPageDoesNotOverflow(page)
    await screenshot(page, testInfo, `${viewport.name}-article`)

    await page.goto('/admin/posts')
    if (viewport.width <= 720) {
      await assertWithinViewport(page, page.getByRole('button', { name: 'Open navigation' }))
      await page.getByRole('button', { name: 'Open navigation' }).click()
      await expect.poll(async () => (await page.locator('.admin-nav').boundingBox())?.x ?? -Infinity).toBeGreaterThanOrEqual(-1)
      await assertWithinViewport(page, page.locator('.admin-nav'))
      await page.getByRole('button', { name: 'Close navigation' }).click()
    } else {
      await assertWithinViewport(page, page.locator('.admin-nav'))
    }
    await assertWithinViewport(page, page.getByRole('heading', { name: 'Posts' }))
    await assertPageDoesNotOverflow(page)
    await screenshot(page, testInfo, `${viewport.name}-admin-list`)

    await page.getByRole('link', { name: visualArticle.title }).click()
    await assertWithinViewport(page, page.locator('#title'))
    await assertWithinViewport(page, page.locator('[data-testid="markdown-editor"]'))
    await assertWithinViewport(page, page.locator('.publish-actions'))
    await assertPageDoesNotOverflow(page)
    await screenshot(page, testInfo, `${viewport.name}-editor`)
  }
})
