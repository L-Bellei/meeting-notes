import { useRef, useEffect } from "react"

interface Props {
  playing: boolean
}

export function AudioSpectrumVisualizer({ playing }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const rafRef = useRef(0)
  const stateRef = useRef({
    targets: Array.from({ length: 22 }, () => 0),
    current: Array.from({ length: 22 }, () => 4),
  })

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const g = canvas.getContext("2d")
    if (!g) return

    cancelAnimationFrame(rafRef.current)

    const barW = 4
    const gap = 3
    const count = Math.min(stateRef.current.current.length, Math.floor(canvas.width / (barW + gap)))

    if (!playing) {
      g.clearRect(0, 0, canvas.width, canvas.height)
      g.fillStyle = "rgba(99,102,241,0.25)"
      for (let i = 0; i < count; i++) {
        g.beginPath()
        g.roundRect(i * (barW + gap), canvas.height - 4, barW, 4, 2)
        g.fill()
      }
      return
    }

    const s = stateRef.current
    const draw = () => {
      for (let i = 0; i < count; i++) {
        if (Math.random() < 0.08) {
          s.targets[i] = 4 + Math.random() * (canvas.height - 4)
        }
        s.current[i] += (s.targets[i] - s.current[i]) * 0.18
      }

      g.clearRect(0, 0, canvas.width, canvas.height)
      for (let i = 0; i < count; i++) {
        const h = Math.max(4, s.current[i])
        const y = canvas.height - h
        const grad = g.createLinearGradient(0, y, 0, canvas.height)
        grad.addColorStop(0, "rgba(129,140,248,0.9)")
        grad.addColorStop(1, "rgba(79,70,229,0.9)")
        g.fillStyle = grad
        g.beginPath()
        g.roundRect(i * (barW + gap), y, barW, h, 2)
        g.fill()
      }
      rafRef.current = requestAnimationFrame(draw)
    }

    draw()
    return () => cancelAnimationFrame(rafRef.current)
  }, [playing])

  return (
    <canvas
      ref={canvasRef}
      width={180}
      height={32}
      className="w-full"
      style={{ height: 32 }}
    />
  )
}
