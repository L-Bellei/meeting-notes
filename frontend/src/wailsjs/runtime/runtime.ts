// stub for local dev before wails generate
export function EventsOn(event: string, cb: (data: any) => void): () => void {
  if (typeof (window as any).runtime?.EventsOn === "function") {
    return (window as any).runtime.EventsOn(event, cb)
  }
  return () => {}
}
