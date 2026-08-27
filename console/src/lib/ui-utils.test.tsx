import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"

import { highlightSearch } from "./ui-utils"

describe("highlightSearch", () => {
    it("wraps every match in a <strong> element", () => {
        const { container } = render(<span>{highlightSearch("order.created.order", "order")}</span>)

        const matches = container.querySelectorAll("strong.match")
        expect(matches).toHaveLength(2)
        expect(container.textContent).toBe("order.created.order")
    })

    it("renders HTML in the text as literal characters", () => {
        const payload = `<img src=x onerror="alert(1)">signup`
        const { container } = render(<span>{highlightSearch(payload, "signup")}</span>)

        expect(container.querySelector("img")).toBeNull()
        expect(container.textContent).toBe(payload)
        expect(screen.getByText("signup").tagName).toBe("STRONG")
    })

    it("renders HTML in the search term as literal characters", () => {
        const search = "<script>alert(1)</script>"
        const { container } = render(<span>{highlightSearch(`a${search}b`, search)}</span>)

        expect(container.querySelector("script")).toBeNull()
        expect(container.textContent).toBe(`a${search}b`)
    })

    it("returns the text unchanged when there is no search term", () => {
        const { container } = render(<span>{highlightSearch("order.created", "")}</span>)

        expect(container.querySelector("strong")).toBeNull()
        expect(container.textContent).toBe("order.created")
    })

    it("uses the provided match class name", () => {
        const { container } = render(<span>{highlightSearch("signup", "sign", "hit")}</span>)

        expect(container.querySelector("strong.hit")?.textContent).toBe("sign")
    })
})
