// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import { initApi } from "../../hooks/useApi"
import { SearchModal } from "./SearchModal"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe("SearchModal", () => {
  it("exposes a named modal dialog", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <SearchModal onClose={vi.fn()} onSelect={vi.fn()} />
      </QueryClientProvider>,
    )

    expect(screen.getByRole("dialog", { name: "Buscar reuniões" })).toBeTruthy()
  })

  it("announces search errors", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    initApi(12345)
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")))

    render(
      <QueryClientProvider client={queryClient}>
        <SearchModal onClose={vi.fn()} onSelect={vi.fn()} />
      </QueryClientProvider>,
    )

    await userEvent.type(screen.getByPlaceholderText("Buscar em reuniões..."), "ab")
    expect(await screen.findByRole("alert")).toBeTruthy()
  })
})
