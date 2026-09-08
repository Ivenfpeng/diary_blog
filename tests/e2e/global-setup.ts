import { execFileSync, spawn, type ChildProcess } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { FullConfig } from '@playwright/test'
import { e2eBaseURL, e2eListenAddress } from './config'

const username = 'release-gate-admin'
const password = 'release-gate-password'

async function waitForReady(process: ChildProcess): Promise<void> {
  const deadline = Date.now() + 30_000
  while (Date.now() < deadline) {
    if (process.exitCode !== null) throw new Error(`Blog server exited with code ${process.exitCode}`)
    try {
      const response = await fetch(`${e2eBaseURL}/readyz`)
      if (response.status === 204) return
    } catch {
      // The server is still starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 150))
  }
  throw new Error('Blog server did not become ready within 30 seconds')
}

export default async function globalSetup(_config: FullConfig): Promise<() => Promise<void>> {
  const runtimeDir = mkdtempSync(join(tmpdir(), 'diary-blog-e2e-'))
  const dataDir = join(runtimeDir, 'data')
  const binary = join(runtimeDir, 'blog')
  const env = {
    ...process.env,
    BLOG_ADDR: e2eListenAddress(),
    BLOG_DATA_DIR: dataDir,
    BLOG_PUBLIC_URL: e2eBaseURL,
    GOCACHE: process.env.GOCACHE ?? '/tmp/diary-blog-go-cache',
  }
  execFileSync('go', ['build', '-o', binary, './cmd/blog'], {
    cwd: process.cwd(),
    env,
    stdio: 'pipe',
  })
  execFileSync(binary, ['admin', 'reset-password', '--username', username], {
    cwd: process.cwd(),
    env,
    input: `${password}\n`,
    stdio: 'pipe',
  })
  const server = spawn(binary, ['serve'], { cwd: process.cwd(), env, stdio: 'pipe' })
  try {
    await waitForReady(server)
  } catch (error) {
    server.kill('SIGTERM')
    rmSync(runtimeDir, { force: true, recursive: true })
    throw error
  }
  return async () => {
    if (server.exitCode === null) {
      await new Promise<void>((resolve) => {
        server.once('exit', () => resolve())
        server.kill('SIGTERM')
      })
    }
    rmSync(runtimeDir, { force: true, recursive: true })
  }
}

export { password, username }
