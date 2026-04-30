import { useQuery } from "@tanstack/react-query"
import { api } from "./useApi"

export interface SearchResultItem {
  meeting_id: string
  meeting_title: string
  snippet: string
  started_at: string | null
  status: string
}

export function useSearch(q: string) {
  return useQuery({
    queryKey: ["search", q],
    queryFn: () => api<SearchResultItem[]>(`/api/search?q=${encodeURIComponent(q)}`),
    enabled: q.trim().length >= 2,
    staleTime: 0,
  })
}
