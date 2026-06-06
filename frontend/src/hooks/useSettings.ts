import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"

export interface Settings {
  user_name: string
  ai_provider: "anthropic" | "openai"
  anthropic_api_key: string
  anthropic_model: string
  openai_api_key: string
  openai_model: string
  auto_generate: string
  whisper_language: string
  whisper_model: string
  recording_hotkey: string
  meeting_name_template: string
}

export function useSettings() {
  return useQuery({
    queryKey: ["settings"],
    queryFn: () => api<Settings>("/api/settings"),
  })
}

export function useUpdateSettings() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<Settings>) =>
      api<Settings>("/api/settings", { method: "PUT", body: JSON.stringify(data) }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["settings"] })
      qc.invalidateQueries({ queryKey: ["ai-health"] })
    },
  })
}
