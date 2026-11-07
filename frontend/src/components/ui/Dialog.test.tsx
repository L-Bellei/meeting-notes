// @vitest-environment jsdom

import { useState } from "react"
import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import { Dialog } from "./Dialog"

afterEach(cleanup)

describe("Dialog", () => {
  it("traps keyboard focus, closes with Escape, and restores focus", async () => {
    const user = userEvent.setup()

    function Harness() {
      const [open, setOpen] = useState(false)

      return (
        <>
          <button onClick={() => setOpen(true)}>Abrir</button>
          {open && (
            <Dialog
              open
              onClose={() => setOpen(false)}
              labelledBy="dialog-title"
              overlayClassName="bg-black/50"
              panelClassName="bg-background"
            >
              <h2 id="dialog-title">Diálogo de teste</h2>
              <button>Primeiro</button>
              <button>Último</button>
            </Dialog>
          )}
        </>
      )
    }

    render(<Harness />)
    const trigger = screen.getByRole("button", { name: "Abrir" })
    trigger.focus()
    await user.click(trigger)

    const dialog = screen.getByRole("dialog", { name: "Diálogo de teste" })
    const first = screen.getByRole("button", { name: "Primeiro" })
    const last = screen.getByRole("button", { name: "Último" })

    expect(document.activeElement).toBe(first)
    await user.tab()
    expect(document.activeElement).toBe(last)
    await user.tab()
    expect(document.activeElement).toBe(first)
    await user.tab({ shift: true })
    expect(document.activeElement).toBe(last)
    trigger.focus()
    fireEvent.keyDown(trigger, { key: "Tab" })
    expect(document.activeElement).toBe(first)

    fireEvent.keyDown(last, { key: "Escape" })
    expect(screen.queryByRole("dialog")).toBeNull()
    expect(document.activeElement).toBe(trigger)
    expect(dialog).toBeTruthy()
  })

  it("closes on backdrop clicks only when enabled", () => {
    const onClose = vi.fn()
    render(
      <Dialog
        open
        onClose={onClose}
        labelledBy="dialog-title"
        overlayClassName="bg-black/50"
        panelClassName="bg-background"
        closeOnBackdrop
      >
        <h2 id="dialog-title">Diálogo</h2>
      </Dialog>,
    )

    fireEvent.click(screen.getByRole("dialog").parentElement!)
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("does not close on backdrop clicks by default", () => {
    const onClose = vi.fn()
    render(
      <Dialog
        open
        onClose={onClose}
        labelledBy="dialog-title"
        overlayClassName="bg-black/50"
        panelClassName="bg-background"
      >
        <h2 id="dialog-title">Diálogo</h2>
      </Dialog>,
    )

    fireEvent.click(screen.getByRole("dialog").parentElement!)
    expect(onClose).not.toHaveBeenCalled()
  })
})
