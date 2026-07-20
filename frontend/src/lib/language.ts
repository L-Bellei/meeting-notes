const LABELS: Record<string, string> = {
  pt: "Português",
  en: "Inglês",
  es: "Espanhol",
  fr: "Francês",
  de: "Alemão",
  it: "Italiano",
  ja: "Japonês",
  zh: "Chinês",
}

export function languageLabel(code: string): string {
  return LABELS[code.toLowerCase()] ?? code.toUpperCase()
}
