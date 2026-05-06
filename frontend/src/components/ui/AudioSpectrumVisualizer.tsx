import { useRef, useEffect, RefObject } from "react"

interface Props {
  audioRef: RefObject<HTMLAudioElement>
  playing: boolean
}

export function AudioSpectrumVisualizer({ audioRef, playing }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const ctxRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const rafRef = useRef(0)

  // Wire up Web Audio API once
  useEffect(() => {
    const audio = audioRef.current
    if (!audio || ctxRef.current) return
    const audioCtx = new AudioContext()
    const analyser = audioCtx.createAnalyser()
    analyser.fftSize = 64 // 32 frequency buckets
    const source = audioCtx.createMediaElementSource(audio)
    source.connect(analyser)
    analyser.connect(audioCtx.destination)
    ctxRef.current = audioCtx
    analyserRef.current = analyser
  }, [audioRef])

  // Draw loop
  useEffect(() => {
    const canvas = canvasRef.current
    const analyser = analyserRef.current
    if (!canvas || !analyser) return

    if (!playing) {
      cancelAnimationFrame(rafRef.current)
      // Clear to flat bars
      const g = canvas.getContext("2d")
      if (g) {
        g.clearRect(0, 0, canvas.width, canvas.height)
        const barW = 4
        const gap = 3
        const count = Math.floor(canvas.width / (barW + gap))
        g.fillStyle = "rgba(99,102,241,0.25)"
        for (let i = 0; i < count; i++) {
          g.beginPath()
          g.roundRect(i * (barW + gap), canvas.height - 4, barW, 4, 2)
          g.fill()
        }
      }
      return
    }

    ctxRef.current?.resume()

    const draw = () => {
      if (!analyser || !canvas) return
      const g = canvas.getContext("2d")
      if (!g) return

      const data = new Uint8Array(analyser.frequencyBinCount)
      analyser.getByteFrequencyData(data)

      g.clearRect(0, 0, canvas.width, canvas.height)

      const barW = 4
      const gap = 3
      const count = Math.min(data.length, Math.floor(canvas.width / (barW + gap)))

      for (let i = 0; i < count; i++) {
        const ratio = data[i] / 255
        const h = Math.max(4, ratio * canvas.height)
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
