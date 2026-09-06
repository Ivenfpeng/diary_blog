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

async function assertWithinViewport(page: Page, locator: Locator, options: { checkVertical?: boolean } = {}): Promise<void> {
  await expect(locator).toBeVisible()
  await locator.scrollIntoViewIfNeeded()
  const box = await locator.boundingBox()
  expect(box, `${await locator.evaluate((element) => element.className || element.tagName)} has a layout box`).not.toBeNull()
  expect(box!.x).toBeGreaterThanOrEqual(-1)
  expect(box!.y).toBeGreaterThanOrEqual(-1)
  expect(box!.x + box!.width).toBeLessThanOrEqual(page.viewportSize()!.width + 1)
  if (options.checkVertical) {
    expect(box!.y + box!.height).toBeLessThanOrEqual(page.viewportSize()!.height + 1)
  }
}

async function assertCenterIsNotOccluded(page: Page, locator: Locator): Promise<void> {
  await locator.scrollIntoViewIfNeeded()
  const box = await locator.boundingBox()
  expect(box, 'element has a box for occlusion testing').not.toBeNull()
  const viewport = page.viewportSize()
  expect(viewport, 'page has a viewport').not.toBeNull()
  const point = {
    x: Math.min(Math.max(box!.x + box!.width / 2, 1), viewport!.width - 1),
    y: Math.min(Math.max(box!.y + Math.min(box!.height / 2, 48), 1), viewport!.height - 1),
  }
  const receivesPointer = await locator.evaluate((element, center) => {
    const topElement = document.elementFromPoint(center.x, center.y)
    return topElement === element || element.contains(topElement)
  }, point)
  expect(receivesPointer, `${await locator.evaluate((element) => element.className || element.tagName)} is not covered at its visible center`).toBe(true)
}

async function assertNoOverlap(page: Page, first: Locator, second: Locator, label: string): Promise<void> {
  const [firstBox, secondBox] = await Promise.all([first.boundingBox(), second.boundingBox()])
  expect(firstBox, `${label}: first element has a layout box`).not.toBeNull()
  expect(secondBox, `${label}: second element has a layout box`).not.toBeNull()
  const overlaps = !(
    firstBox!.x + firstBox!.width <= secondBox!.x + 1 ||
    secondBox!.x + secondBox!.width <= firstBox!.x + 1 ||
    firstBox!.y + firstBox!.height <= secondBox!.y + 1 ||
    secondBox!.y + secondBox!.height <= firstBox!.y + 1
  )
  expect(overlaps, label).toBe(false)
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
    await assertWithinViewport(page, page.locator('.site-header'), { checkVertical: true })
    await assertCenterIsNotOccluded(page, page.locator('.site-header'))
    await assertWithinViewport(page, page.getByRole('heading', { name: 'Latest writing' }), { checkVertical: true })
    await assertNoOverlap(page, page.locator('.site-header'), page.getByRole('heading', { name: 'Latest writing' }), `${viewport.name} public header does not overlap the listing title`)
    if (viewport.width <= 800) await assertWithinViewport(page, page.locator('.mobile-navigation summary'), { checkVertical: true })
    else await assertWithinViewport(page, page.locator('.header-nav'), { checkVertical: true })
    await assertPageDoesNotOverflow(page)
    await screenshot(page, testInfo, `${viewport.name}-home`)

    await page.goto(`/posts/${visualArticle.slug}`)
    await assertWithinViewport(page, page.locator('.article-header h1'), { checkVertical: true })
    await assertCenterIsNotOccluded(page, page.locator('.article-header h1'))
    await assertWithinViewport(page, page.locator('.prose'))
    await assertWithinViewport(page, page.locator('.prose pre'), { checkVertical: true })
    if (viewport.width <= 800) await assertWithinViewport(page, page.locator('.mobile-table-of-contents'), { checkVertical: true })
    else {
      await assertWithinViewport(page, page.locator('.desktop-table-of-contents'), { checkVertical: true })
      await assertNoOverlap(page, page.locator('.prose'), page.locator('.desktop-table-of-contents'), `${viewport.name} article body does not overlap the table of contents`)
    }
    await assertPageDoesNotOverflow(page)
    await screenshot(page, testInfo, `${viewport.name}-article`)

    await page.goto('/admin/posts')
    const adminTopbar = page.locator('.admin-topbar')
    if (viewport.width <= 720) {
      await assertWithinViewport(page, page.getByRole('button', { name: 'Open navigation' }), { checkVertical: true })
      await page.getByRole('button', { name: 'Open navigation' }).click()
      await expect.poll(async () => (await page.locator('.admin-nav').boundingBox())?.x ?? -Infinity).toBeGreaterThanOrEqual(-1)
      await assertWithinViewport(page, page.locator('.admin-nav'), { checkVertical: true })
      await assertCenterIsNotOccluded(page, page.locator('.admin-nav'))
      await assertNoOverlap(page, adminTopbar, page.locator('.admin-nav'), `${viewport.name} mobile admin topbar does not cover open navigation`)
      await page.getByRole('button', { name: 'Close navigation' }).click()
      await expect.poll(async () => (await page.locator('.admin-nav').boundingBox())?.x ?? 0).toBeLessThanOrEqual(-220)
    } else {
      await assertWithinViewport(page, page.locator('.admin-nav'), { checkVertical: true })
      await assertNoOverlap(page, page.locator('.admin-nav'), page.getByRole('heading', { name: 'Posts' }), `${viewport.name} admin sidebar does not overlap content`)
    }
    await assertWithinViewport(page, page.getByRole('heading', { name: 'Posts' }), { checkVertical: true })
    await assertCenterIsNotOccluded(page, page.getByRole('heading', { name: 'Posts' }))
    await assertPageDoesNotOverflow(page)
    await screenshot(page, testInfo, `${viewport.name}-admin-list`)

    await page.getByRole('link', { name: visualArticle.title }).click()
    await assertWithinViewport(page, page.locator('#title'), { checkVertical: true })
    await assertCenterIsNotOccluded(page, page.locator('#title'))
    await assertWithinViewport(page, page.locator('[data-testid="markdown-editor"]'), { checkVertical: true })
    await assertWithinViewport(page, page.locator('.publish-actions'), { checkVertical: true })
    await assertCenterIsNotOccluded(page, page.locator('.publish-actions'))
    await assertNoOverlap(page, page.locator('#title'), page.locator('[data-testid="markdown-editor"]'), `${viewport.name} editor title does not overlap Markdown editor`)
    await assertNoOverlap(page, page.locator('[data-testid="markdown-editor"]'), page.locator('.publish-actions'), `${viewport.name} Markdown editor does not overlap publish controls`)
    await assertPageDoesNotOverflow(page)
    await screenshot(page, testInfo, `${viewport.name}-editor`)
  }
})
