import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Journey, Project } from "@/types"
import type { UUID } from "@/types/common"
import { actionStep } from "./Action"

vi.mock("react-i18next", () => ({
    useTranslation: () => ({
        t: (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
    }),
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

vi.mock("@/components/action-preview-iframe", () => ({ ActionPreviewIframe: () => null }))

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>

const project = { id: "p1" } as Project
const journey = { id: "j1" } as Journey
const Describe = actionStep.Describe!

describe("action step", () => {
    beforeAll(() => {
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("loads an action and the action types once for every step that uses it", async () => {
        vi.mocked(oapiClient.GET).mockImplementation(
            (async (path: string) =>
                ({
                    data: path.endsWith("/meta")
                        ? [{ type: "webhook", functions: [{ id: "post", title: "Post" }] }]
                        : { id: "a1", name: "Slack hook", type: "webhook" },
                }) as unknown as ClientResult) as never,
        )

        const value = { action_id: "a1" as UUID, function_id: "post" }
        render(
            <QueryClientProvider client={queryClient}>
                {["s1", "s2", "s3"].map((id) => (
                    <Describe
                        key={id}
                        project={project}
                        journey={journey}
                        value={value}
                        onChange={() => {}}
                    />
                ))}
            </QueryClientProvider>,
        )

        expect((await screen.findAllByText(/Post/)).length).toBe(3)
        expect(vi.mocked(oapiClient.GET)).toHaveBeenCalledTimes(2)
    })
})
