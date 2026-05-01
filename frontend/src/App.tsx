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
import { Spinner } from "./components/ui/spinner"
import { BoardView } from "./components/board/BoardView"
import { SearchModal } from "./components/search/SearchModal"
import { useSettings } from "./hooks/useSettings"
import { formatHotkey } from "./lib/formatHotkey"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 10_000 } },
})

function AppInner() {
  const [ready, setReady] = useState(false)
  const [startupError, setStartupError] = useState(false)
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
    GetPort().then(async port => {
      initApi(port)
      const deadline = Date.now() + 15_000
      while (Date.now() < deadline) {
        try {
          const res = await fetch(`http://localhost:${port}/health`)
          if (res.ok) { if (!cancelled) setReady(true); return }
        } catch {
          // server not up yet
        }
        await new Promise(r => setTimeout(r, 500))
      }
      if (!cancelled) setStartupError(true)
    })
    return () => { cancelled = true }
  }, [])

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

  function handleSearchSelect(meetingId: string, query: string) {
    setSelectedMeetingId(meetingId)
    setHighlightQuery(query)
    setActiveView("meetings")
  }

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center flex-col gap-3 text-muted-foreground text-sm animate-fade-in">
        {startupError ? (
          <p className="text-destructive text-center px-8">
            Não foi possível conectar ao servidor. Tente reiniciar o app.
          </p>
        ) : (
          <>
            <Spinner size={24} className="text-primary" />
            Iniciando...
          </>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <Toolbar
        onToggleSidebar={() => setSidebarOpen(o => !o)}
        onRecord={() => setRecordingModalOpen(true)}
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
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppInner />
    </QueryClientProvider>
  )
}
