import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import { UserImportDialog } from "./user-import-dialog"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)
vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
    Trans: ({ children }: { children?: unknown }) => children,
}))
vi.mock("i18next", () => ({ t: translate }))

describe("UserImportDialog", () => {
    it("shows a failed import and keeps the dialog open", async () => {
        const onOpenChange = vi.fn()
        const onImport = vi.fn().mockRejectedValue(new Error("network down"))
        render(<UserImportDialog open onOpenChange={onOpenChange} onImport={onImport} />)

        const input = document.querySelector("input[type=file]") as HTMLInputElement
        fireEvent.change(input, {
            target: { files: [new File(["external_id\n1"], "users.csv", { type: "text/csv" })] },
        })
        await userEvent.click(screen.getByRole("button", { name: /Import/ }))

        expect(await screen.findByText("Couldn't import the users.")).toBeTruthy()
        expect(onOpenChange).not.toHaveBeenCalledWith(false)
    })

    it("closes after a successful import", async () => {
        const onOpenChange = vi.fn()
        const onImport = vi.fn().mockResolvedValue(undefined)
        render(<UserImportDialog open onOpenChange={onOpenChange} onImport={onImport} />)

        const input = document.querySelector("input[type=file]") as HTMLInputElement
        fireEvent.change(input, {
            target: { files: [new File(["external_id\n1"], "users.csv", { type: "text/csv" })] },
        })
        await userEvent.click(screen.getByRole("button", { name: /Import/ }))

        await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    })
})
