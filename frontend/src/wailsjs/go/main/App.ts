// stub for local dev before wails generate
export async function GetPort(): Promise<number> {
  return (window as any).go?.main?.App?.GetPort?.() ?? 0
}
