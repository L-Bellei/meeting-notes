import { useState, useEffect, useRef } from "react"
import { createPortal } from "react-dom"
import { Search, X } from "lucide-react"
import { useSearch } from "../../hooks/useSearch"
import { Spinner } from "../ui/spinner"

interface Props {
  onClose: () => void
  onSelect: (meetingId: string, query: string) => void
}

export function SearchModal({ onClose, onSelect }: Props) {
  const [input, setInput] = useState("")
  const [q, setQ] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)
  const { data: results, isFetching } = useSearch(q)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  useEffect(() => {
    const timer = setTimeout(() => setQ(input), 200)
    return () => clearTimeout(timer)
  }, [input])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [onClose])

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-24 bg-black/50"
      onClick={onClose}
    >
      <div
        className="w-full max-w-xl bg-background border border-border rounded-lg shadow-xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
          <Search size={16} className="text-muted-foreground shrink-0" />
          <input
            ref={inputRef}
            value={input}
            onChange={e => setInput(e.target.value)}
            placeholder="Buscar em reuniões..."
            className="flex-1 bg-transparent outline-none text-sm"
          />
          {isFetching && <Spinner size={14} />}
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X size={16} />
          </button>
        </div>

        {q.trim().length >= 2 && (
          <ul className="max-h-80 overflow-y-auto py-1">
            {results && results.length === 0 && (
              <li className="px-4 py-3 text-sm text-muted-foreground">
                Nenhum resultado para "{q}"
              </li>
            )}
            {results?.map(item => (
              <li key={item.meeting_id}>
                <button
                  className="w-full text-left px-4 py-3 hover:bg-accent transition-colors"
                  onClick={() => { onSelect(item.meeting_id, q); onClose() }}
                >
                  <p className="text-sm font-medium truncate">{item.meeting_title}</p>
                  <p
                    className="text-xs text-muted-foreground mt-0.5 line-clamp-2 [&_b]:font-semibold [&_b]:text-foreground"
                    dangerouslySetInnerHTML={{ __html: item.snippet }}
                  />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>,
    document.body,
  )
}
