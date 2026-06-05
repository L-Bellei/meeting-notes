import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { GetPort } from "./wailsjs/go/main/App"
import { EventsOn } from "./wailsjs/runtime/runtime"
import { initApi } from "./hooks/useApi"
import { usePipeline } from "./hooks/usePipeline"
import { Sidebar } from "./components/layout/Sidebar"
import { MeetingList } from "./components/layout/MeetingList"
import { MeetingDetail } from "./components/layout/MeetingDetail"
import { Toolbar } from "./components/layout/Toolbar"
import { RecordingModal } from "./components/recording/RecordingModal"
import { StopConfirmModal } from "./components/recording/StopConfirmModal"
import { SettingsModal } from "./components/settings/SettingsModal"
import { LoadingScreen } from "./components/ui/LoadingScreen"
import type { LoadingCheck } from "./components/ui/LoadingScreen"
import { BoardView } from "./components/board/BoardView"
import { SearchModal } from "./components/search/SearchModal"
import { useSettings } from "./hooks/useSettings"
import { formatHotkey } from "./lib/formatHotkey"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 10_000 } },
})

function AppInner() {
  const [ready, setReady] = useState(false)
  const [fadingOut, setFadingOut] = useState(false)
  const [showLoading, setShowLoading] = useState(true)
  const [checks, setChecks] = useState<LoadingCheck[]>([
    { label: "Servidor HTTP",          status: "loading" },
    { label: "Modelo de transcrição",  status: "pending" },
    { label: "Chave da API Anthropic", status: "hidden"  },
  ])
  const [selectedThemeId, setSelectedThemeId] = useState<string | null>(null)
  const [selectedMeetingId, setSelectedMeetingId] = useState<string | null>(null)
  const [recordingModalOpen, setRecordingModalOpen] = useState(false)
  const [stopConfirmMeetingId, setStopConfirmMeetingId] = useState<string | null>(null)
  const [hotkeySuggestedTitle, setHotkeySuggestedTitle] = useState<string>("")
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [activeView, setActiveView] = useState<"meetings" | "board">("meetings")
  const [searchOpen, setSearchOpen] = useState(false)
  const [highlightQuery, setHighlightQuery] = useState<string | undefined>(undefined)

  const { data: settings } = useSettings()
  const recordingHotkey = formatHotkey(settings?.recording_hotkey ?? "ctrl+shift+r")

  useEffect(() => {
    let cancelled = false

    const upd = (index: number, patch: Partial<LoadingCheck>) =>
      setChecks(prev => prev.map((c, i) => i === index ? { ...c, ...patch } : c))

    GetPort().then(async port => {
      initApi(port)

      // Check 1 — Servidor HTTP
      const serverDeadline = Date.now() + 15_000
      let serverOk = false
      while (Date.now() < serverDeadline) {
        try {
          const res = await fetch(`http://localhost:${port}/health`)
          if (res.ok) { serverOk = true; break }
        } catch { /* aguarda */ }
        await new Promise(r => setTimeout(r, 500))
      }
      if (cancelled) return
      if (!serverOk) {
        upd(0, { status: "error", error: "Não foi possível conectar ao servidor. Tente reiniciar o app." })
        return
      }
      upd(0, { status: "done" })
      upd(1, { status: "loading" })

      // Check 2 — Modelo de transcrição
      const modelDeadline = Date.now() + 120_000
      let modelOk = false
      while (Date.now() < modelDeadline) {
        try {
          const res = await fetch(`http://localhost:${port}/health`)
          if (res.ok) {
            const h = await res.json() as { model_loaded?: boolean }
            if (h.model_loaded) { modelOk = true; break }
          }
        } catch { /* aguarda */ }
        await new Promise(r => setTimeout(r, 2_000))
      }
      if (cancelled) return
      if (!modelOk) {
        upd(1, { status: "error", error: "O modelo de transcrição demorou muito. Gravações funcionam, mas a transcrição pode falhar." })
      } else {
        upd(1, { status: "done" })
      }

      // Check 3 — Chave da API (condicional)
      try {
        const aiHealth = await fetch(`http://localhost:${port}/api/ai/health`).then(r => r.json()) as
          { configured: boolean; valid?: boolean; error?: string }

        if (aiHealth.configured) {
          if (!cancelled) upd(2, { status: "loading" })
          await new Promise(r => setTimeout(r, 0))
          if (!cancelled) {
            if (aiHealth.valid) {
              upd(2, { status: "done" })
            } else {
              upd(2, { status: "error", error: aiHealth.error ?? "Chave inválida. Verifique nas configurações." })
            }
          }
        }
      } catch { /* silencioso */ }

      if (!cancelled) setReady(true)
    })

    return () => { cancelled = true }
  }, [])

  useEffect(() => {
    if (!ready) return
    setFadingOut(true)
    const t = setTimeout(() => setShowLoading(false), 300)
    return () => clearTimeout(t)
  }, [ready])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "k" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault()
        setSearchOpen(true)
      }
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [])

  useEffect(() => {
    const unlisten = EventsOn("hotkey:recording-started", ({ meetingId }: { meetingId: string }) => {
      setSelectedMeetingId(meetingId)
      setHighlightQuery(undefined)
      setActiveView("meetings")
    })
    return () => { if (typeof unlisten === "function") unlisten() }
  }, [])

  useEffect(() => {
    const unlisten = EventsOn("hotkey:open-recording-modal", ({ suggestedTitle }: { suggestedTitle: string }) => {
      setHotkeySuggestedTitle(suggestedTitle)
      setRecordingModalOpen(true)
    })
    return () => { if (typeof unlisten === "function") unlisten() }
  }, [])

  useEffect(() => {
    const unlisten = EventsOn("hotkey:confirm-stop", ({ meetingId }: { meetingId: string }) => {
      setStopConfirmMeetingId(meetingId)
    })
    return () => { if (typeof unlisten === "function") unlisten() }
  }, [])

  usePipeline()

  function applyTemplate(): string {
    const tpl = settings?.meeting_name_template ?? "Reunião {date}"
    const now = new Date()
    const dd   = String(now.getDate()).padStart(2, "0")
    const mm   = String(now.getMonth() + 1).padStart(2, "0")
    const yyyy = now.getFullYear()
    const hh   = String(now.getHours()).padStart(2, "0")
    const min  = String(now.getMinutes()).padStart(2, "0")
    return tpl.replace("{date}", `${dd}/${mm}/${yyyy}`).replace("{time}", `${hh}:${min}`)
  }

  function handleSearchSelect(meetingId: string, query: string) {
    setSelectedMeetingId(meetingId)
    setHighlightQuery(query)
    setActiveView("meetings")
  }

  return (
    <>
      {showLoading && <LoadingScreen checks={checks} fading={fadingOut} />}
      {ready && (
        <div className="flex flex-col h-screen overflow-hidden bg-background">
          <Toolbar
            onToggleSidebar={() => setSidebarOpen(o => !o)}
            onRecord={() => { setHotkeySuggestedTitle(applyTemplate()); setRecordingModalOpen(true) }}
            onSettings={() => setSettingsOpen(true)}
            onSearch={() => setSearchOpen(true)}
            recordingHotkey={recordingHotkey}
            activeView={activeView}
            onChangeView={setActiveView}
          />
          <Sidebar
            open={sidebarOpen}
            onClose={() => setSidebarOpen(false)}
            selectedThemeId={selectedThemeId}
            onSelectTheme={setSelectedThemeId}
          />
          <div className="flex flex-1 overflow-hidden">
            {activeView === "board" ? (
              <BoardView />
            ) : (
              <>
                <MeetingList
                  themeId={selectedThemeId}
                  selectedMeetingId={selectedMeetingId}
                  onSelectMeeting={id => { setSelectedMeetingId(id); setHighlightQuery(undefined) }}
                  onMeetingDeleted={id => { if (selectedMeetingId === id) setSelectedMeetingId(null) }}
                  onOpenSearch={() => setSearchOpen(true)}
                />
                <MeetingDetail
                  meetingId={selectedMeetingId}
                  onDeleted={() => setSelectedMeetingId(null)}
                  highlightQuery={highlightQuery}
                  onOpenSettings={() => setSettingsOpen(true)}
                />
              </>
            )}
          </div>
          {searchOpen && (
            <SearchModal
              onClose={() => setSearchOpen(false)}
              onSelect={handleSearchSelect}
            />
          )}
          <RecordingModal
            open={recordingModalOpen}
            onClose={() => { setRecordingModalOpen(false); setHotkeySuggestedTitle("") }}
            onMeetingCreated={(id: string) => setSelectedMeetingId(id)}
            initialTitle={hotkeySuggestedTitle}
          />
          <StopConfirmModal
            open={stopConfirmMeetingId !== null}
            meetingId={stopConfirmMeetingId}
            onClose={() => setStopConfirmMeetingId(null)}
          />
          <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
        </div>
      )}
    </>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppInner />
    </QueryClientProvider>
  )
}
