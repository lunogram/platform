import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import { InlineEdit } from "./inline-edit"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: translate }) }))

describe("InlineEdit", () => {
    afterEach(() => {
        vi.restoreAllMocks()
    })

    it("keeps the editor open with the user's value when saving fails", async () => {
        vi.spyOn(console, "error").mockImplementation(() => {})
        const onSave = vi.fn().mockRejectedValue(new Error("name taken"))
        render(<InlineEdit value="Old name" onSave={onSave} />)

        await userEvent.click(screen.getByRole("button", { name: "Edit value" }))
        const input = screen.getByRole("textbox") as HTMLInputElement
        await userEvent.clear(input)
        await userEvent.type(input, "New name")
        await userEvent.click(screen.getByRole("button", { name: "save" }))

        await waitFor(() => expect(onSave).toHaveBeenCalledWith("New name"))
        await waitFor(() =>
            expect(
                (screen.getByRole("button", { name: "save" }) as HTMLButtonElement).disabled,
            ).toBe(false),
        )
        expect((screen.getByRole("textbox") as HTMLInputElement).value).toBe("New name")
    })

    it("closes the editor after a successful save", async () => {
        const onSave = vi.fn().mockResolvedValue(undefined)
        render(<InlineEdit value="Old name" onSave={onSave} />)

        await userEvent.click(screen.getByRole("button", { name: "Edit value" }))
        const input = screen.getByRole("textbox")
        await userEvent.clear(input)
        await userEvent.type(input, "New name")
        await userEvent.click(screen.getByRole("button", { name: "save" }))

        await waitFor(() => expect(screen.queryByRole("textbox")).toBeNull())
    })
})
