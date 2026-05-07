import { createPortal } from "react-dom"
import { useRef, useState, useCallback, useEffect } from "react"
import { X, Play, Pause } from "lucide-react"
import { getApiBase } from "../../hooks/useApi"
import { AudioSpectrumVisualizer } from "./AudioSpectrumVisualizer"
import { cn } from "../../lib/utils"

interface Props {
  meetingId: string
  meetingTitle: string
  onClose: () => void
}

function formatTime(s: number): string {
  if (!isFinite(s)) return "0:00"
  const m = Math.floor(s / 60)
  const sec = Math.floor(s % 60)
  return `${m}:${sec.toString().padStart(2, "0")}`
}

export function AudioPlayer({ meetingId, meetingTitle, onClose }: Props) {
  const audioRef = useRef<HTMLAudioElement>(null)
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)

  // Drag state — uses translate() to avoid bottom/right → left/top coordinate conversion
  const dragRef = useRef<{ startMouseX: number; startMouseY: number; baseX: number; baseY: number } | null>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [translate, setTranslate] = useState({ x: 0, y: 0 })

  const src = `${getApiBase()}/api/meetings/${meetingId}/audio`

  const toggle = useCallback(() => {
    const a = audioRef.current
    if (!a) return
    a.paused ? a.play().catch(() => {}) : a.pause()
  }, [])

  const skip = useCallback((delta: number) => {
    const a = audioRef.current
    if (!a) return
    a.currentTime = Math.max(0, Math.min(a.duration || 0, a.currentTime + delta))
  }, [])

  const seek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const a = audioRef.current
    if (!a || !a.duration) return
    const rect = e.currentTarget.getBoundingClientRect()
    a.currentTime = ((e.clientX - rect.left) / rect.width) * a.duration
  }, [])

  // Drag handlers
  function onHandleMouseDown(e: React.MouseEvent<HTMLDivElement>) {
    if (e.button !== 0) return
    dragRef.current = {
      startMouseX: e.clientX,
      startMouseY: e.clientY,
      baseX: translate.x,
      baseY: translate.y,
    }
    setIsDragging(true)
    e.preventDefault()
  }

  useEffect(() => {
    function onMove(e: MouseEvent) {
      if (!dragRef.current) return
      setTranslate({
        x: dragRef.current.baseX + (e.clientX - dragRef.current.startMouseX),
        y: dragRef.current.baseY + (e.clientY - dragRef.current.startMouseY),
      })
    }
    function onUp() {
      if (!dragRef.current) return
      dragRef.current = null
      setIsDragging(false)
    }
    document.addEventListener("mousemove", onMove)
    document.addEventListener("mouseup", onUp)
    return () => {
      document.removeEventListener("mousemove", onMove)
      document.removeEventListener("mouseup", onUp)
    }
  }, [])

  const progress = duration > 0 ? currentTime / duration : 0

  const content = (
    <div
      style={{ transform: `translate(${translate.x}px, ${translate.y}px)` }}
      className="fixed bottom-4 right-4 z-[9999] w-52 rounded-2xl border border-border bg-card shadow-2xl shadow-black/50 select-none"
      onDragStart={e => e.preventDefault()}
    >
      <audio
        ref={audioRef}
        src={src}
        onTimeUpdate={() => setCurrentTime(audioRef.current?.currentTime ?? 0)}
        onLoadedMetadata={() => setDuration(audioRef.current?.duration ?? 0)}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => setPlaying(false)}
      />

      {/* Drag handle — header bar */}
      <div
        className={cn(
          "flex items-center justify-between px-3 pt-3 pb-2",
          isDragging ? "cursor-grabbing" : "cursor-grab"
        )}
        onMouseDown={onHandleMouseDown}
      >
        <p className="text-[11px] text-muted-foreground font-medium truncate pr-2 flex-1">
          {meetingTitle}
        </p>
        <button
          onMouseDown={e => e.stopPropagation()}
          onClick={onClose}
          className="flex-shrink-0 text-muted-foreground hover:text-foreground transition-colors"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="px-4 pb-4">
        {/* seek bar */}
        <div
          className="h-1 w-full rounded-full bg-muted cursor-pointer mb-1.5 relative"
          onClick={seek}
        >
          <div
            className="h-full rounded-full bg-primary transition-none"
            style={{ width: `${progress * 100}%` }}
          />
          <div
            className="absolute top-1/2 -translate-y-1/2 w-3 h-3 rounded-full bg-primary/80 shadow -ml-1.5"
            style={{ left: `${progress * 100}%` }}
          />
        </div>
        <div className="flex justify-between text-[10px] text-muted-foreground mb-3">
          <span>{formatTime(currentTime)}</span>
          <span>{formatTime(duration)}</span>
        </div>

        {/* controls */}
        <div className="flex items-center justify-center gap-3 mb-3">
          <button
            onClick={() => skip(-15)}
            className="text-[11px] font-semibold text-muted-foreground hover:text-foreground bg-muted px-2 py-1 rounded-md transition-colors"
          >
            −15s
          </button>
          <button
            onClick={toggle}
            className={cn(
              "w-9 h-9 rounded-full bg-primary flex items-center justify-center text-primary-foreground shadow-md",
              "hover:bg-primary/90 transition-colors"
            )}
          >
            {playing
              ? <Pause className="h-4 w-4" />
              : <Play className="h-4 w-4 ml-0.5" />}
          </button>
          <button
            onClick={() => skip(15)}
            className="text-[11px] font-semibold text-muted-foreground hover:text-foreground bg-muted px-2 py-1 rounded-md transition-colors"
          >
            +15s
          </button>
        </div>

        {/* spectrum */}
        <div className="border-t border-border/50 pt-3">
          <AudioSpectrumVisualizer playing={playing} />
        </div>
      </div>
    </div>
  )

  return createPortal(content, document.body)
}
