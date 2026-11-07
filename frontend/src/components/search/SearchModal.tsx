import { useState, useEffect } from "react"
import { Search, X } from "lucide-react"
import { useSearch } from "../../hooks/useSearch"
import { Spinner } from "../ui/spinner"
import { Dialog } from "../ui/Dialog"

interface Props {
  onClose: () => void
  onSelect: (meetingId: string, query: string) => void
}

export function SearchModal({ onClose, onSelect }: Props) {
  const [input, setInput] = useState("")
  const [q, setQ] = useState("")
  const { data: results, isFetching, isError, error } = useSearch(q)

  useEffect(() => {
    const timer = setTimeout(() => setQ(input), 200)
    return () => clearTimeout(timer)
  }, [input])

  return (
    <Dialog
      open
      onClose={onClose}
      labelledBy="search-modal-title"
      overlayClassName="items-start pt-24 bg-black/50"
      panelClassName="w-full max-w-xl bg-background border border-border rounded-lg shadow-xl overflow-hidden"
      closeOnBackdrop
    >
      <div>
        <h2 id="search-modal-title" className="sr-only">Buscar reuniões</h2>
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
          <Search size={16} className="text-muted-foreground shrink-0" />
          <input
            autoFocus
            value={input}
            onChange={e => setInput(e.target.value)}
            placeholder="Buscar em reuniões..."
            aria-label="Buscar em reuniões"
            className="flex-1 bg-transparent outline-none text-sm"
          />
          {isFetching && (
            <>
              <span role="status" className="sr-only">Buscando reuniões...</span>
              <span aria-hidden="true"><Spinner size={14} /></span>
            </>
          )}
          <button
            type="button"
            onClick={onClose}
            aria-label="Fechar busca"
            className="text-muted-foreground hover:text-foreground"
          >
            <X size={16} />
          </button>
        </div>

        {q.trim().length >= 2 && (
          <ul className="max-h-80 overflow-y-auto py-1">
            {isError && (
              <li role="alert" className="px-4 py-3 text-sm text-destructive">
                Não foi possível buscar reuniões: {error.message}
              </li>
            )}
            {results && results.length === 0 && (
              <li role="status" className="px-4 py-3 text-sm text-muted-foreground">
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
    </Dialog>
  )
}
