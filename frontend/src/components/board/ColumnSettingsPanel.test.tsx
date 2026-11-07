// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { ColumnSettingsPanel } from "./ColumnSettingsPanel"

vi.mock("../../hooks/useBoardColumns", () => ({
  useColumns: () => ({
    data: [
      { id: "column-1", name: "A fazer", card_count: 1, position: 0 },
      { id: "column-2", name: "Concluído", card_count: 0, position: 1 },
    ],
  }),
  useCreateColumn: () => ({ mutate: vi.fn() }),
  useUpdateColumn: () => ({ mutate: vi.fn() }),
  useDeleteColumn: () => ({ mutate: vi.fn(), isPending: false }),
  useReorderColumns: () => ({ mutate: vi.fn() }),
}))

afterEach(cleanup)

describe("ColumnSettingsPanel", () => {
  it("keeps the delete confirmation inside the existing dialog", () => {
    render(<ColumnSettingsPanel onClose={vi.fn()} />)

    fireEvent.click(screen.getByRole("button", { name: "Excluir coluna A fazer" }))

    expect(screen.getAllByRole("dialog")).toHaveLength(1)
    expect(screen.getByRole("group", { name: /A fazer/ })).toBeTruthy()
    expect(screen.getByRole("combobox", { name: "Coluna de destino" })).toBeTruthy()
  })
})
