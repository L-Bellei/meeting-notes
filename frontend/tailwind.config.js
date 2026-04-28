/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    colors: {
      transparent: "transparent",
      current: "currentColor",
      background:  "#111111",
      foreground:  "#ede9ff",
      border:      "#2a2a2a",
      input:       "#1c1c1c",
      primary: {
        DEFAULT:    "#7c3aed",
        foreground: "#ffffff",
      },
      muted: {
        DEFAULT:    "#1c1c1c",
        foreground: "#8b7aaa",
      },
      accent: {
        DEFAULT:    "#222222",
        foreground: "#ede9ff",
      },
      destructive: {
        DEFAULT:    "#ef4444",
        foreground: "#ffffff",
      },
      purple: {
        400: "#a78bfa",
        500: "#8b5cf6",
        600: "#7c3aed",
        700: "#6d28d9",
        900: "#2e1065",
      },
      red:    { 500: "#ef4444", 700: "#b91c1c" },
      yellow: { 400: "#facc15", 500: "#eab308" },
      green:  { 500: "#22c55e" },
      gray:   { 400: "#6b7280", 600: "#4b5563" },
      black:  "#000000",
      white:  "#ffffff",
    },
    extend: {
      borderRadius: {
        lg:   "0.75rem",
        md:   "0.5rem",
        sm:   "0.375rem",
        xl:   "1rem",
        "2xl": "1.25rem",
        "3xl": "1.5rem",
      },
      keyframes: {
        wave: {
          "0%, 100%": { transform: "scaleY(0.4)" },
          "50%":       { transform: "scaleY(1.2)" },
        },
        "fade-in": {
          "0%":   { opacity: "0", transform: "translateY(6px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
      },
      animation: {
        wave1: "wave 1.2s ease-in-out infinite 0s",
        wave2: "wave 1.2s ease-in-out infinite 0.15s",
        wave3: "wave 1.2s ease-in-out infinite 0.3s",
        wave4: "wave 1.2s ease-in-out infinite 0.45s",
        wave5: "wave 1.2s ease-in-out infinite 0.6s",
        "fade-in": "fade-in 0.35s ease-out forwards",
      },
    },
  },
  plugins: [require("@tailwindcss/typography")],
}
