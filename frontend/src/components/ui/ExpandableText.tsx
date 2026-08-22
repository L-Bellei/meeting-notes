import { useEffect, useRef, useState } from "react"
import { cn } from "../../lib/utils"

interface Props {
  text: string
  lines: number
  className?: string
}

export function ExpandableText({ text, lines, className }: Props) {
  const [expanded, setExpanded] = useState(false)
  const [overflows, setOverflows] = useState(false)
  const ref = useRef<HTMLParagraphElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    // Medido no elemento cortado: com o texto expandido scrollHeight == clientHeight
    // e a checagem daria falso negativo, então só medimos enquanto está cortado.
    if (expanded) return
    setOverflows(el.scrollHeight > el.clientHeight + 1)
  }, [text, lines, expanded])

  const clamp = expanded
    ? undefined
    : {
        display: "-webkit-box" as const,
        WebkitBoxOrient: "vertical" as const,
        WebkitLineClamp: lines,
        overflow: "hidden" as const,
      }

  return (
    <div>
      <p
        ref={ref}
        style={clamp}
        className={cn("text-sm text-muted-foreground whitespace-pre-wrap leading-relaxed", className)}
      >
        {text}
      </p>
      {(overflows || expanded) && (
        <button
          onClick={() => setExpanded(v => !v)}
          className="text-xs text-primary hover:underline mt-1"
        >
          {expanded ? "ver menos" : "ver mais"}
        </button>
      )}
    </div>
  )
}
