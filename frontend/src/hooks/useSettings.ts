import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api, useApiReady } from "./useApi"

export interface Settings {
  user_name: string
  claude_code_token: string
  claude_code_model: string
  auto_generate: string
  whisper_language: string
  whisper_model: string
  keep_audio: string
  recording_hotkey: string
  meeting_name_template: string
  sidebar_pinned: string
}

// O GET devolve todas as linhas da tabela, inclusive chaves legadas que o PUT
// recusa (ai_provider). Enviar só o que o backend aceita.
export const WRITABLE_SETTINGS: (keyof Settings)[] = [
  "user_name",
  "claude_code_token",
  "claude_code_model",
  "auto_generate",
  "whisper_language",
  "whisper_model",
  "keep_audio",
  "recording_hotkey",
  "meeting_name_template",
  "sidebar_pinned",
]

export function pickWritable(form: Partial<Settings>): Partial<Settings> {
  const out: Partial<Settings> = {}
  for (const key of WRITABLE_SETTINGS) {
    const value = form[key]
    if (value !== undefined) out[key] = value
  }
  return out
}

export function useSettings() {
  const apiReady = useApiReady()
  return useQuery({
    queryKey: ["settings"],
    queryFn: () => api<Settings>("/api/settings"),
    enabled: apiReady,
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
