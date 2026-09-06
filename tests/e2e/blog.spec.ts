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
  await page.getByLabel('Alt text').fill('A green release-gate pixel')
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
  await page.locator('#category').fill(categoryID)
  await fillMarkdown(page, page.locator('.cm-content'), `${articleMarkdown}\n![Release-gate image](${uploadedImage})\n`)
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
  await fillMarkdown(page, page.locator('.cm-content'), `${articleMarkdown}\n\nThis revision is public.\n`)
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
  await page.locator('#category').fill(categoryID)
  await fillMarkdown(page, page.locator('.cm-content'), '# Private release draft\nThis must remain private.')
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
