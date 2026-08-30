import { useQuery } from "@tanstack/react-query"
import { api, useApiReady } from "./useApi"

export interface AudioHealth {
  status: string
  model_loaded: boolean
  gpu_available: boolean
  gpu_name: string | null
  gpu_vram_mb: number | null
  gpu_vendor: "nvidia" | "amd" | "intel" | "other" | null
  gpu_backend: "cuda" | "vulkan" | null
  vulkan_model_ready: boolean
  device: string
}

export function useAudioHealth() {
  const apiReady = useApiReady()
  return useQuery({
    queryKey: ["audio-health"],
    queryFn: () => api<AudioHealth>("/health"),
    enabled: apiReady,
    staleTime: 30_000,
  })
}
