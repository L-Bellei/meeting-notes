import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api, getApiBase } from "./useApi"
import { useSettings } from "./useSettings"

interface AIHealth {
  configured: boolean
  valid?: boolean
  error?: string
}

export function useAIHealth() {
  return useQuery({
    queryKey: ["ai-health"],
    queryFn: () => api<AIHealth>("/api/ai/health"),
    staleTime: 5 * 60_000,
  })
}

// useAIConfigured exposes whether the AI provider is usable.
// `configured` is derived locally from settings (instant, gates the UI);
// `valid` comes from the backend Ping (true=token works, false=token rejected).
export function useAIConfigured() {
  const { data: settings } = useSettings()
  const { data: health, isFetching } = useAIHealth()

  const configured = Boolean(settings?.claude_code_token?.trim())

  return {
    configured,
    valid: health?.valid,
    checkError: health?.error,
    checking: isFetching,
  }
}

export function useClaudeLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      const res = await fetch(`${getApiBase()}/api/ai/claude-login`, { method: "POST" })
      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: res.statusText }))
        if (res.status === 503) {
          throw new Error(
            "Claude Code não encontrado. Instale o Claude Code (npm i -g @anthropic-ai/claude-code) e tente novamente."
          )
        }
        throw new Error(body.error ?? "não foi possível abrir o login")
      }
      return (await res.json()) as { status: string }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["settings"] })
      qc.invalidateQueries({ queryKey: ["ai-health"] })
    },
  })
}

export function useAITest() {
  return useMutation({
    mutationFn: () => api<{ ok: boolean }>("/api/ai/test", { method: "POST" }),
  })
}
