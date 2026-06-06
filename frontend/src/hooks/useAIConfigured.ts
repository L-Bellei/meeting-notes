import { useQuery } from "@tanstack/react-query"
import { api } from "./useApi"
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
// `valid` comes from the backend Ping (true=key works, false=key rejected).
export function useAIConfigured() {
  const { data: settings } = useSettings()
  const { data: health, isFetching } = useAIHealth()

  const provider = settings?.ai_provider
  const key =
    provider === "openai" ? settings?.openai_api_key : settings?.anthropic_api_key
  const configured = Boolean(provider && key)

  return {
    configured,
    valid: health?.valid,
    checkError: health?.error,
    checking: isFetching,
  }
}
