export const e2ePort = process.env.BLOG_E2E_PORT ?? '18080'
export const e2eBaseURL = process.env.BLOG_E2E_BASE_URL ?? `http://127.0.0.1:${e2ePort}`

export function e2eListenAddress(): string {
  const url = new URL(e2eBaseURL)
  const port = url.port || (url.protocol === 'https:' ? '443' : '80')
  return `${url.hostname}:${port}`
}
