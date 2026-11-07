import { useEffect, useRef, type ReactNode } from "react"
import { createPortal } from "react-dom"
import { cn } from "../../lib/utils"

interface Props {
  open: boolean
  onClose: () => void
  labelledBy: string
  describedBy?: string
  overlayClassName: string
  panelClassName: string
  closeOnBackdrop?: boolean
  closeOnEscape?: boolean
  onEscape?: (event: KeyboardEvent) => void
  children: ReactNode
}

const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  'input:not([disabled]):not([type="hidden"])',
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(",")

export function Dialog({
  open,
  onClose,
  labelledBy,
  describedBy,
  overlayClassName,
  panelClassName,
  closeOnBackdrop = false,
  closeOnEscape = true,
  onEscape,
  children,
}: Props) {
  const panelRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)
  const onEscapeRef = useRef(onEscape)

  useEffect(() => {
    onCloseRef.current = onClose
    onEscapeRef.current = onEscape
  }, [onClose, onEscape])

  useEffect(() => {
    if (!open) return

    const previousFocus = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null
    const panel = panelRef.current
    const focusable = panel?.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)
    const initialFocus = panel?.querySelector<HTMLElement>("[autofocus]") ?? focusable?.[0] ?? panel
    initialFocus?.focus()

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && closeOnEscape && !event.defaultPrevented) {
        event.preventDefault()
        if (onEscapeRef.current) {
          onEscapeRef.current(event)
        } else {
          onCloseRef.current()
        }
        return
      }

      if (event.key !== "Tab" || !panel) return
      const currentFocusable = Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
      if (currentFocusable.length === 0) {
        event.preventDefault()
        panel.focus()
        return
      }

      const currentIndex = currentFocusable.indexOf(document.activeElement as HTMLElement)
      if (currentIndex === -1) {
        event.preventDefault()
        const target = event.shiftKey ? currentFocusable[currentFocusable.length - 1] : currentFocusable[0]
        target.focus()
      } else if (event.shiftKey && currentIndex === 0) {
        event.preventDefault()
        currentFocusable[currentFocusable.length - 1].focus()
      } else if (!event.shiftKey && currentIndex === currentFocusable.length - 1) {
        event.preventDefault()
        currentFocusable[0].focus()
      }
    }

    window.addEventListener("keydown", onKeyDown)
    return () => {
      window.removeEventListener("keydown", onKeyDown)
      if (previousFocus?.isConnected) previousFocus.focus()
    }
  }, [open, closeOnEscape])

  if (!open) return null

  return createPortal(
    <div
      className={cn("fixed inset-0 z-50 flex items-center justify-center", overlayClassName)}
      onClick={event => {
        if (closeOnBackdrop && event.target === event.currentTarget) onCloseRef.current()
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        aria-describedby={describedBy}
        tabIndex={-1}
        className={panelClassName}
      >
        {children}
      </div>
    </div>,
    document.body,
  )
}
