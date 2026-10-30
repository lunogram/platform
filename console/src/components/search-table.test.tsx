import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { describe, expect, it, vi } from "vitest"
import type { SearchParams, SearchResult } from "@/types"
import { SearchTable } from "./search-table"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: translate }) }))

interface Row {
    id: string
    name: string
}

const columns = [{ key: "name" }]
const page2: SearchResult<Row> = {
    results: [{ id: "r1", name: "Ada" }],
    nextCursor: "c3",
    prevCursor: "c1",
    limit: 15,
}

describe("SearchTable", () => {
    it("starts a new search from the first page", async () => {
        const params: SearchParams = { limit: 15, cursor: "c2", page: "prev", search: "" }
        const setParams = vi.fn()
        render(
            <SearchTable<Row>
                columns={columns}
                results={page2}
                params={params}
                setParams={setParams}
                enableSearch
            />,
        )

        await userEvent.type(screen.getByRole("textbox"), "ada")

        await waitFor(() => expect(setParams).toHaveBeenCalled(), { timeout: 2000 })
        const next = setParams.mock.calls.at(-1)![0] as SearchParams
        expect(next.search).toBe("ada")
        expect(next.cursor).toBeUndefined()
        expect(next.page).toBe("next")
    })

    it("shows a failed load with Retry instead of a skeleton", async () => {
        const onRetry = vi.fn()
        render(
            <SearchTable<Row>
                columns={columns}
                results={null}
                params={{ limit: 15 }}
                setParams={() => {}}
                isError
                onRetry={onRetry}
            />,
        )

        expect(screen.getByText("Couldn't load results.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(onRetry).toHaveBeenCalledTimes(1)
    })

    it("shows the empty message when told loading is done without results", () => {
        render(
            <SearchTable<Row>
                columns={columns}
                results={null}
                params={{ limit: 15 }}
                setParams={() => {}}
                isLoading={false}
                emptyMessage="Nothing here"
            />,
        )

        expect(screen.getByText("Nothing here")).toBeTruthy()
    })
})
