/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    colors: {
      border: "#e5e4e7",
      background: "#ffffff",
      foreground: "#08060d",
      primary: {
        DEFAULT: "#1c1a24",
        foreground: "#fafbfc",
      },
      muted: {
        DEFAULT: "#f5f5f7",
        foreground: "#766d7b",
      },
      destructive: {
        DEFAULT: "#dc2626",
        foreground: "#fafbfc",
      },
      accent: {
        DEFAULT: "#f5f5f7",
        foreground: "#1c1a24",
      },
      input: "#e5e4e7",
      white: "#ffffff",
      red: {
        500: "#ef4444",
        700: "#b91c1c",
      },
      yellow: {
        400: "#facc15",
        500: "#eab308",
      },
      green: {
        500: "#22c55e",
      },
      gray: {
        400: "#9ca3af",
      },
    },
    extend: {
      borderRadius: {
        lg: "0.5rem",
        md: "calc(0.5rem - 2px)",
        sm: "calc(0.5rem - 4px)",
      },
    },
  },
}
