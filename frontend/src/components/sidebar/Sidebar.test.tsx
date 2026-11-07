// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { Sidebar } from "./Sidebar"

const { updateTheme, themes } = vi.hoisted(() => ({
  updateTheme: vi.fn(),
  themes: [
    {
      id: "parent",
      parent_id: null,
      name: "Pai",
      description: "",
      color: "#123456",
      custom_prompt: "",
      auto_add_to_board: false,
      created_at: "",
    },
    {
      id: "child",
      parent_id: "parent",
      name: "Filho",
      description: "",
      color: "#123456",
      custom_prompt: "",
      auto_add_to_board: false,
      created_at: "",
    },
    {
      id: "movable",
      parent_id: null,
      name: "Movível",
      description: "",
      color: "#123456",
      custom_prompt: "",
      auto_add_to_board: false,
      created_at: "",
    },
  ],
}))

vi.mock("../../hooks/useThemes", () => ({
  useThemes: () => ({ data: themes }),
  useDeleteTheme: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useUpdateTheme: () => ({ mutateAsync: updateTheme }),
}))

vi.mock("../../hooks/useMeetings", () => ({
  useMeetings: () => ({ data: [] }),
}))

vi.mock("../../hooks/useSidebarPinned", () => ({
  useSidebarPinned: () => ({ pinned: true, ready: true, toggle: vi.fn() }),
}))

vi.mock("../../hooks/useThemeExpanded", () => ({
  useThemeExpanded: () => ({
    expanded: { parent: true },
    toggle: vi.fn(),
    expand: vi.fn(),
  }),
}))

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  vi.restoreAllMocks()
})

function mockThemeRowsLayout() {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (
    this: HTMLElement,
  ) {
    const text = this.textContent ?? ""
    const top = text.includes("Solte aqui")
      ? 0
      : text.includes("Pai")
        ? 40
        : text.includes("Filho")
          ? 80
          : text.includes("Movível")
            ? 120
            : 200
    return {
      x: 0,
      y: top,
      top,
      right: 256,
      bottom: top + 32,
      left: 0,
      width: 256,
      height: 32,
      toJSON: () => ({}),
    }
  })
}

describe("Sidebar theme reparenting", () => {
  it("provides a keyboard-focusable drag handle only for movable themes", () => {
    render(
      <Sidebar
        open
        onClose={vi.fn()}
        selectedThemeId={null}
        onSelectTheme={vi.fn()}
      />,
    )

    expect(screen.getByRole("button", { name: "Mover tema Movível" }).tabIndex).toBe(0)
    expect(screen.getByRole("button", { name: "Mover tema Filho" })).toBeTruthy()
    expect(screen.queryByRole("button", { name: "Mover tema Pai" })).toBeNull()
  })

  it("announces keyboard instructions and cancels without changing the theme", async () => {
    render(
      <Sidebar
        open
        onClose={vi.fn()}
        selectedThemeId={null}
        onSelectTheme={vi.fn()}
      />,
    )
    const handle = screen.getByRole("button", { name: "Mover tema Movível" })
    handle.focus()
    fireEvent.keyDown(handle, { code: "Space", key: " " })
    await waitFor(() => expect(screen.getByRole("status").textContent).toContain("Movendo"))

    const instructionsId = handle.getAttribute("aria-describedby")

    expect(instructionsId).toBeTruthy()
    expect(instructionsId && document.getElementById(instructionsId)?.textContent).toContain(
      "pressione Espaço ou Enter",
    )

    fireEvent.keyDown(handle, { code: "Escape", key: "Escape" })

    await waitFor(() => expect(screen.getByRole("status").textContent).toContain("cancelado"))
    expect(updateTheme).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(handle)
  })

  it("moves a theme to a first-level parent with the keyboard", async () => {
    mockThemeRowsLayout()
    render(
      <Sidebar
        open
        onClose={vi.fn()}
        selectedThemeId={null}
        onSelectTheme={vi.fn()}
      />,
    )
    const handle = screen.getByRole("button", { name: "Mover tema Movível" })

    handle.focus()
    fireEvent.keyDown(handle, { code: "Space", key: " " })
    await waitFor(() => expect(screen.getByRole("status").textContent).toContain("Movendo"))
    fireEvent.keyDown(handle, { code: "ArrowUp", key: "ArrowUp" })
    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain("Movível sobre Pai"),
    )
    fireEvent.keyDown(handle, { code: "Tab", key: "Tab" })

    await waitFor(() =>
      expect(updateTheme).toHaveBeenCalledWith(
        expect.objectContaining({ id: "movable", parent_id: "parent" }),
      ),
    )
    expect(document.activeElement).toBe(handle)
  })

  it("moves a subcategory to the root with the keyboard", async () => {
    mockThemeRowsLayout()
    render(
      <Sidebar
        open
        onClose={vi.fn()}
        selectedThemeId={null}
        onSelectTheme={vi.fn()}
      />,
    )
    const handle = screen.getByRole("button", { name: "Mover tema Filho" })

    handle.focus()
    fireEvent.keyDown(handle, { code: "Space", key: " " })
    await waitFor(() => expect(screen.getByRole("status").textContent).toContain("Movendo"))
    fireEvent.keyDown(handle, { code: "ArrowUp", key: "ArrowUp" })
    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain("Filho sobre Pai"),
    )
    fireEvent.keyDown(handle, { code: "ArrowUp", key: "ArrowUp" })
    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toContain("Filho sobre a raiz"),
    )
    fireEvent.keyDown(handle, { code: "Space", key: " " })

    await waitFor(() =>
      expect(updateTheme).toHaveBeenCalledWith(
        expect.objectContaining({ id: "child", parent_id: null }),
      ),
    )
  })
})
