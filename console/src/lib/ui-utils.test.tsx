import { render, screen } from "@testing-library/react"
import { highlightSearch } from "./ui-utils"

describe("highlightSearch", () => {
    it("highlights all exact matches with strong tags", () => {
        render(<div>{highlightSearch("foo foo", "foo")}</div>)

        expect(screen.getAllByText("foo", { selector: "strong" })).toHaveLength(2)
    })

    it("does not render HTML from untrusted input", () => {
        const payload = `<img src="x" onerror="alert(1)">`
        const { container } = render(<div>{highlightSearch(payload, "img")}</div>)

        expect(container.querySelector("img")).toBeNull()
        expect(screen.getByText("img", { selector: "strong" })).toBeInTheDocument()
        expect(container).toHaveTextContent(payload)
    })

    it("returns plain text when search is empty", () => {
        const { container } = render(<div>{highlightSearch("plain text", "")}</div>)

        expect(screen.getByText("plain text")).toBeInTheDocument()
        expect(container.querySelector("strong")).toBeNull()
    })
})
