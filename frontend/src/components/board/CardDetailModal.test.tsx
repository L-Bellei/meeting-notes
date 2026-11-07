// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { CardDetailModal } from "./CardDetailModal"

const { refetch } = vi.hoisted(() => ({ refetch: vi.fn() }))

vi.mock("../../hooks/useBoard", () => ({
  useCardDetail: () => ({
    data: undefined,
    isLoading: false,
    isError: true,
    isFetching: false,
    error: new Error("network failure"),
    refetch,
  }),
  useUpdateCard: () => ({ isPending: false, mutate: vi.fn() }),
  useLinkCardToMeeting: () => ({ isPending: false, mutate: vi.fn() }),
  useDeleteCard: () => ({ mutate: vi.fn() }),
}))

vi.mock("../../hooks/useMeetings", () => ({
  useMeetings: () => ({ data: [] }),
}))

afterEach(cleanup)

describe("CardDetailModal", () => {
  it("announces loading failures and offers a retry", () => {
    render(<CardDetailModal cardId="card-1" onClose={vi.fn()} />)

    expect(screen.getByRole("dialog", { name: "Detalhes do card" })).toBeTruthy()
    expect(screen.getByRole("alert").textContent).toContain("network failure")
    fireEvent.click(screen.getByRole("button", { name: "Tentar novamente" }))
    expect(refetch).toHaveBeenCalledOnce()
  })
})
