import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"

export interface BoardColumn {
  id: string
  name: string
  position: number
  created_at: string
  card_count: number
}

export function useColumns() {
  return useQuery({
    queryKey: ["board-columns"],
    queryFn: () => api<BoardColumn[]>("/api/board/columns"),
  })
}

export function useCreateColumn() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) =>
      api<BoardColumn>("/api/board/columns", { method: "POST", body: JSON.stringify({ name }) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board-columns"] }),
  })
}

export function useUpdateColumn() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) =>
      api<BoardColumn>(`/api/board/columns/${id}`, { method: "PUT", body: JSON.stringify({ name }) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board-columns"] }),
  })
}

export function useDeleteColumn() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, moveTo }: { id: string; moveTo?: string }) => {
      const url = moveTo ? `/api/board/columns/${id}?move_to=${moveTo}` : `/api/board/columns/${id}`
      return api<void>(url, { method: "DELETE" })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["board-columns"] })
      qc.invalidateQueries({ queryKey: ["board-cards"] })
    },
  })
}

export function useReorderColumns() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (items: Array<{ id: string; position: number }>) =>
      api<void>("/api/board/columns/reorder", { method: "PATCH", body: JSON.stringify(items) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["board-columns"] }),
  })
}
