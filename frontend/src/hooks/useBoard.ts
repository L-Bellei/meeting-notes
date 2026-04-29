import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"

export interface TaskProgress { total: number; completed: number }

export interface BoardCardSummary {
  id: string
  meeting_id: string
  column_id: string
  number: number
  position: number
  description: string
  updated_at: string
  created_at: string
  meeting_title: string
  theme_id: string | null
  theme_name: string | null
  theme_color: string | null
  status: string
  task_progress: TaskProgress
}

export interface BoardCardDetail {
  id: string
  meeting_id: string
  column_id: string
  number: number
  position: number
  description: string
  updated_at: string
  created_at: string
  status: string
  meeting_title: string
  theme_id: string | null
  theme_name: string | null
  theme_color: string | null
  summary: { id: string; content: string; model_used: string } | null
  key_points: Array<{ id: string; content: string; position: number; meeting_id: string }>
  tasks: Array<{ id: string; description: string; completed: boolean; priority: string; assignee: string | null; meeting_id: string }>
}

export interface BoardCardFilters {
  title?: string
  number?: number
  created_after?: string
  created_before?: string
  updated_after?: string
  updated_before?: string
}

export function useCards(filters: BoardCardFilters = {}) {
  const params = new URLSearchParams()
  if (filters.title) params.set("title", filters.title)
  if (filters.number != null) params.set("number", String(filters.number))
  if (filters.created_after) params.set("created_after", filters.created_after)
  if (filters.created_before) params.set("created_before", filters.created_before)
  if (filters.updated_after) params.set("updated_after", filters.updated_after)
  if (filters.updated_before) params.set("updated_before", filters.updated_before)
  const qs = params.toString()
  return useQuery({
    queryKey: ["board-cards", filters],
    queryFn: () => api<BoardCardSummary[]>(`/api/board/cards${qs ? "?" + qs : ""}`),
  })
}

export function useCardDetail(id: string | null) {
  return useQuery({
    queryKey: ["board-card", id],
    queryFn: () => api<BoardCardDetail>(`/api/board/cards/${id}`),
    enabled: !!id,
  })
}

export function useCreateCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ meeting_id, column_id }: { meeting_id: string; column_id?: string }) =>
      api<{ id: string; number: number }>("/api/board/cards", {
        method: "POST",
        body: JSON.stringify({ meeting_id, column_id }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["board-cards"] })
      qc.invalidateQueries({ queryKey: ["board-columns"] })
    },
  })
}

export function useDeleteCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api<void>(`/api/board/cards/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["board-cards"] })
      qc.invalidateQueries({ queryKey: ["board-columns"] })
    },
  })
}

export function useUpdateCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, description }: { id: string; description: string }) =>
      api<{ id: string }>(`/api/board/cards/${id}`, {
        method: "PUT",
        body: JSON.stringify({ description }),
      }),
    onSuccess: (_data, { id }) => {
      qc.invalidateQueries({ queryKey: ["board-cards"] })
      qc.invalidateQueries({ queryKey: ["board-card", id] })
    },
  })
}

export function useCardForMeeting(meetingId: string | null) {
  const result = useQuery({
    queryKey: ["board-cards", {}],
    queryFn: () => api<BoardCardSummary[]>("/api/board/cards"),
    enabled: !!meetingId,
  })
  return {
    ...result,
    data: meetingId ? result.data?.find(c => c.meeting_id === meetingId) : undefined,
  }
}

export function useMoveCard() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, column_id, position }: { id: string; column_id: string; position: number }) =>
      api<void>(`/api/board/cards/${id}/move`, {
        method: "PATCH",
        body: JSON.stringify({ column_id, position }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board-cards"] }),
    onError: () => qc.invalidateQueries({ queryKey: ["board-cards"] }),
  })
}
