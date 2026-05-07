let baseURL = ""

export function getApiBase(): string {
  return baseURL
}

export function initApi(port: number) {
  baseURL = `http://localhost:${port}`
}

export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${baseURL}${path}`, {
    headers: { "Content-Type": "application/json", ...options?.headers },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? res.statusText)
  }
  const text = await res.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}
