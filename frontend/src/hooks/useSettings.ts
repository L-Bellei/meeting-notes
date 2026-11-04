import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api, useApiReady } from "./useApi"

export interface Settings {
  user_name: string
  claude_code_token_configured: boolean
  claude_code_model: string
  auto_generate: string
  whisper_language: string
  whisper_model: string
  whisper_device: string
  keep_audio: string
  recording_hotkey: string
  meeting_name_template: string
  sidebar_pinned: string
}

export type SettingsForm = Partial<Settings & { claude_code_token: string }>

// O GET devolve também chaves legadas que o PUT recusa (ai_provider). Enviar só o que o backend aceita.
type WritableSetting = Exclude<keyof Settings, "claude_code_token_configured"> | "claude_code_token"
export type SettingsUpdate = Partial<Record<WritableSetting, string>>

export const WRITABLE_SETTINGS: WritableSetting[] = [
  "user_name",
  "claude_code_token",
  "claude_code_model",
  "auto_generate",
  "whisper_language",
  "whisper_model",
  "whisper_device",
  "keep_audio",
  "recording_hotkey",
  "meeting_name_template",
  "sidebar_pinned",
]

export function pickWritable(form: SettingsForm): SettingsUpdate {
  const out: SettingsUpdate = {}
  for (const key of WRITABLE_SETTINGS) {
    const value = form[key]
    if (typeof value === "string") out[key] = value
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
    mutationFn: (data: SettingsUpdate) =>
      api<Settings>("/api/settings", { method: "PUT", body: JSON.stringify(data) }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["settings"] })
      qc.invalidateQueries({ queryKey: ["ai-health"] })
    },
  })
}
