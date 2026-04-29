import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"

export interface Theme {
  id: string
  parent_id: string | null
  name: string
  description: string
  color: string
  custom_prompt: string
  created_at: string
}

export function useThemes() {
  return useQuery({ queryKey: ["themes"], queryFn: () => api<Theme[]>("/api/themes") })
}

export function useCreateTheme() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; description: string; color: string; parent_id?: string | null }) =>
      api<Theme>("/api/themes", { method: "POST", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["themes"] }),
  })
}

export function useUpdateTheme() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; name: string; description: string; color: string; parent_id?: string | null; custom_prompt: string }) =>
      api<Theme>(`/api/themes/${data.id}`, { method: "PUT", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["themes"] }),
  })
}

export function useDeleteTheme() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api<void>(`/api/themes/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["themes"] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    },
  })
}
