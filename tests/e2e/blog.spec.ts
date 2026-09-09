import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Locator, type Page } from '@playwright/test'
import { password, username } from './global-setup'

const articleMarkdown = readFileSync(join(import.meta.dirname, 'fixtures/article.md'), 'utf8')
const articleTitle = 'Release gate publishing workflow'
const articleSlug = 'release-gate-publishing-workflow'
const categorySlug = 'release-notes'

async function signIn(page: Page): Promise<void> {
  await page.goto('/admin/login')
  await page.getByLabel('Username').fill(username)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
}

async function fillMarkdown(page: Page, editor: Locator, markdown: string): Promise<void> {
  await editor.click()
  await editor.press(process.platform === 'darwin' ? 'Meta+A' : 'Control+A')
  await page.keyboard.insertText(markdown)
}

async function clickMarkdownBlankArea(page: Page, editor: Locator): Promise<void> {
  await editor.scrollIntoViewIfNeeded()
  const box = await editor.boundingBox()
  if (!box) throw new Error('Markdown editor is not visible')
  const viewport = page.viewportSize()
  const y = viewport ? Math.min(box.y + Math.min(120, box.height - 24), viewport.height - 24) : box.y + 24
  await page.mouse.click(box.x + 24, y)
}

async function expectListingDoesNotExpose(page: Page, path: string, title: string, slug: string): Promise<void> {
  await page.goto(path)
  await expect(page.getByRole('link', { name: title, exact: true })).toHaveCount(0)
  await expect(page.locator('body')).not.toContainText(slug)
}

async function expectSearchDoesNotExpose(page: Page, query: string, title: string): Promise<void> {
  await page.goto(`/search?q=${encodeURIComponent(query)}`)
  await expect(page.getByRole('link', { name: title, exact: true })).toHaveCount(0)
  await expect(page.getByText('No published articles matched that search.')).toBeVisible()
}

async function expectFeedAndSitemapDoNotExpose(page: Page, title: string, slug: string): Promise<void> {
  await page.goto('/rss.xml')
  await expect(page.locator('body')).not.toContainText(title)
  await page.goto('/sitemap.xml')
  await expect(page.locator('body')).not.toContainText(slug)
}

test('administrator can type Markdown after clicking the blank editor area', async ({ page }) => {
  await signIn(page)

  await page.getByRole('link', { name: 'Posts' }).click()
  await page.getByRole('button', { name: 'New post' }).click()
  await page.locator('#title').fill('空白区域 Markdown 输入！')
  await expect(page.locator('#slug')).toHaveValue('空白区域-markdown-输入')

  await clickMarkdownBlankArea(page, page.locator('.rich-editor-surface'))
  await page.keyboard.insertText('# Typed from blank area\n\nThe editor should accept input here.')
  await page.evaluate(() => {
    const bytes = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL2VwAAAABJRU5ErkJggg=='), (character) => character.charCodeAt(0))
    const data = new DataTransfer()
    data.items.add(new File([bytes], 'inline-editor.png', { type: 'image/png' }))
    const event = new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true })
    document.querySelector('.rich-editor-surface')?.dispatchEvent(event)
  })

  await expect(page.locator('.rich-editor-surface')).toContainText('Typed from blank area')
  await expect(page.locator('.rich-editor-surface img[alt="inline editor"]')).toBeVisible()
  await page.getByRole('button', { name: 'Save now' }).click()
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
})

test('administrator can create taxonomy entries with Chinese slugs', async ({ page }) => {
  await signIn(page)

  await page.getByRole('link', { name: 'Categories & tags' }).click()
  const taxonomyForm = page.locator('.taxonomy-form')
  await taxonomyForm.getByLabel('Name').fill('事实上')
  const slugInput = taxonomyForm.getByLabel('Slug')
  await slugInput.fill('事实-2026')
  await expect.poll(() => slugInput.evaluate((input) => (input as HTMLInputElement).validity.valid)).toBe(true)
  await expect.poll(() => slugInput.evaluate((input) => (input as HTMLInputElement).validationMessage)).toBe('')
  await taxonomyForm.getByRole('button', { name: 'Add' }).click()

  await expect(page.locator('.taxonomy-row').filter({ hasText: '事实上 /事实-2026' })).toBeVisible()
})

test('administrator can publish, revise, restore, and remove a complete article workflow', async ({ page }) => {
  await signIn(page)

  await page.getByRole('link', { name: 'Categories & tags' }).click()
  await page.getByLabel('Name').fill('Release notes')
  await page.getByLabel('Slug').fill(categorySlug)
  await page.getByRole('button', { name: 'Add' }).click()
  const categoryRow = page.locator('.taxonomy-row').filter({ hasText: `Release notes /${categorySlug}` })
  await expect(categoryRow).toBeVisible()
  const categoryEditID = await categoryRow.getByRole('button', { name: 'Edit' }).getAttribute('data-testid')
  const categoryID = categoryEditID?.match(/^edit-categories-(\d+)$/)?.[1]
  if (!categoryID) throw new Error(`Unable to determine release-note category ID from ${categoryEditID}`)

  await page.getByRole('link', { name: 'Media' }).click()
  await page.getByLabel('Image').setInputFiles({
    name: 'release-gate.png',
    mimeType: 'image/png',
    buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL2VwAAAABJRU5ErkJggg==', 'base64'),
  })
  await page.locator('#alt-text').fill('A green release-gate pixel')
  await page.getByRole('button', { name: 'Upload image' }).click()
  await expect(page.getByText('Upload complete.')).toBeVisible()
  const uploadedImage = await page.locator('.media-row img').first().getAttribute('src')
  expect(uploadedImage).toMatch(/^\/media\//)

  await page.getByRole('link', { name: 'Posts' }).click()
  await page.getByRole('button', { name: 'New post' }).click()
  await expect(page.getByRole('heading', { name: 'Untitled article' })).toBeVisible()
  await page.locator('#title').fill(articleTitle)
  await page.locator('#slug').fill(articleSlug)
  await page.locator('#summary').fill('A browser-verified release workflow.')
  await page.locator('#category').selectOption(categoryID)
  await fillMarkdown(page, page.locator('.rich-editor-surface'), `${articleMarkdown}\n![Release-gate image](${uploadedImage})\n`)
  await page.getByRole('button', { name: 'Save now' }).click()
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'Preview' }).click()
  await expect(page.getByRole('heading', { name: 'Preview' })).toBeVisible()
  await expect(page.locator('.preview')).toContainText('Why this workflow exists')

  await page.getByRole('button', { name: 'Publish' }).click()
  await expect(page.getByText('published', { exact: true })).toBeVisible()

  await page.goto(`/posts/${articleSlug}`)
  await expect(page.getByRole('heading', { name: articleTitle })).toBeVisible()
  await expect(page.locator('.article-meta').getByRole('link', { name: 'Release notes' })).toBeVisible()
  await expect(page.locator('.prose img[alt="Release-gate image"]')).toBeVisible()

  await page.goto('/search?q=durable')
  await expect(page.getByRole('link', { name: articleTitle, exact: true })).toBeVisible()
  await page.goto(`/categories/${categorySlug}`)
  await expect(page.getByRole('link', { name: articleTitle, exact: true })).toBeVisible()

  await page.goto('/admin/posts')
  await page.locator('.admin-content .post-row').filter({ hasText: articleTitle }).click()
  const revisedTitle = 'Release gate publishing workflow, revised'
  await page.locator('#title').fill(revisedTitle)
  await fillMarkdown(page, page.locator('.rich-editor-surface'), `${articleMarkdown}\n\nThis revision is public.\n`)
  await page.getByRole('button', { name: 'Save now' }).click()
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Publish' }).click()
  await expect(page.getByText('published', { exact: true })).toBeVisible()

  await page.locator('.revision-panel li').filter({ hasText: articleTitle }).first().getByRole('button', { name: 'Restore' }).click()
  await expect(page.locator('#title')).toHaveValue(articleTitle)
  await page.getByRole('button', { name: 'Publish' }).click()
  await expect(page.getByText('published', { exact: true })).toBeVisible()
  await page.goto(`/posts/${articleSlug}`)
  await expect(page.getByRole('heading', { name: articleTitle })).toBeVisible()
  await expect(page.getByText('This revision is public.')).toHaveCount(0)

  await page.goto('/admin/posts')
  await page.getByRole('button', { name: 'New post' }).click()
  const draftTitle = 'Private release draft'
  const draftSlug = 'private-release-draft'
  await page.locator('#title').fill(draftTitle)
  await page.locator('#slug').fill(draftSlug)
  await page.locator('#category').selectOption(categoryID)
  await fillMarkdown(page, page.locator('.rich-editor-surface'), '# Private release draft\nThis must remain private.')
  await page.getByRole('button', { name: 'Save now' }).click()
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()

  await page.goto(`/posts/${draftSlug}`)
  await expect(page.getByRole('heading', { name: 'Nothing here' })).toBeVisible()
  await expectListingDoesNotExpose(page, '/', draftTitle, draftSlug)
  await expectListingDoesNotExpose(page, `/categories/${categorySlug}`, draftTitle, draftSlug)
  await expectListingDoesNotExpose(page, '/archive', draftTitle, draftSlug)
  await expectSearchDoesNotExpose(page, draftSlug, draftTitle)
  await expectFeedAndSitemapDoNotExpose(page, draftTitle, draftSlug)

  await page.goto('/admin/posts')
  await page.locator('.admin-content .post-row').filter({ hasText: articleTitle }).click()
  await page.getByRole('button', { name: 'Archive' }).click()
  await expect(page.getByText('archived', { exact: true })).toBeVisible()
  await page.goto(`/posts/${articleSlug}`)
  await expect(page.getByRole('heading', { name: 'Nothing here' })).toBeVisible()
  await expectListingDoesNotExpose(page, '/', articleTitle, articleSlug)
  await expectListingDoesNotExpose(page, `/categories/${categorySlug}`, articleTitle, articleSlug)
  await expectListingDoesNotExpose(page, '/archive', articleTitle, articleSlug)
  await expectSearchDoesNotExpose(page, articleTitle, articleTitle)
  await expectFeedAndSitemapDoNotExpose(page, articleTitle, articleSlug)
})
