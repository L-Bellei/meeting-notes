import { useSyncExternalStore } from "react"

let baseURL = ""
const listeners = new Set<() => void>()

export function getApiBase(): string {
  return baseURL
}

export function initApi(port: number) {
  const next = `http://localhost:${port}`
  if (next === baseURL) return
  baseURL = next
  listeners.forEach(listener => listener())
}

function subscribe(callback: () => void): () => void {
  listeners.add(callback)
  return () => listeners.delete(callback)
}

function getReadySnapshot(): boolean {
  return baseURL !== ""
}

export function useApiReady(): boolean {
  return useSyncExternalStore(subscribe, getReadySnapshot)
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
