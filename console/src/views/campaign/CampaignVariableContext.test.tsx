import { describe, expect, it, vi, beforeEach } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import { CampaignContext, ProjectContext } from "@/contexts"
import type { VariableSuggestions } from "@/types"
import { toDisplayConditions } from "./template/mail/editor/blockEditor/displayConditions"

const fetchPathSuggestions = vi.hoisted(() => vi.fn())
vi.mock("@/lib/path-suggestions", () => ({ fetchPathSuggestions }))

const { CampaignVariableProvider, useCampaignVariableContext } =
    await import("./CampaignVariableContext")

/**
 * The schema endpoint prefixes discovered attributes with `.data`, so a boolean
 * user attribute arrives looking like this.
 */
const suggestions = {
    userPaths: [
        { path: ".email", types: ["string"] },
        { path: ".data.isPro", types: ["boolean"] },
        { path: ".data.admin", types: ["boolean"] },
        { path: ".data.plan", types: ["string"] },
    ],
    eventPaths: [],
    scheduledPaths: [],
    organizationEventPaths: [],
    organizationUserPaths: [],
    organizationPaths: [],
} as unknown as VariableSuggestions

function Probe() {
    const { variableGroups, variablesReady } = useCampaignVariableContext()
    const conditions = toDisplayConditions(variableGroups)
    return (
        <div>
            <span data-testid="ready">{String(variablesReady)}</span>
            <span data-testid="conditions">{conditions.map((c) => c.before).join("|")}</span>
        </div>
    )
}

function renderProbe() {
    return render(
        <ProjectContext.Provider value={[{ id: "p1" }] as never}>
            <CampaignContext.Provider value={[{ channel: "email", variables: [] }] as never}>
                <CampaignVariableProvider>
                    <Probe />
                </CampaignVariableProvider>
            </CampaignContext.Provider>
        </ProjectContext.Provider>,
    )
}

describe("CampaignVariableProvider", () => {
    beforeEach(() => {
        fetchPathSuggestions.mockReset()
    })

    // The regression this guards: the block editor reads its display conditions
    // once, at init. Mounting it before the schema fetch settles handed it an
    // empty list — none of the hardcoded base variables is a boolean — and the
    // editor hides the whole control when the list is empty.
    it("reports no variables ready until the schema fetch settles", async () => {
        let settle: (value: VariableSuggestions) => void = () => {}
        fetchPathSuggestions.mockReturnValue(
            new Promise<VariableSuggestions>((resolve) => {
                settle = resolve
            }),
        )

        renderProbe()

        expect(screen.getByTestId("ready")).toHaveTextContent("false")
        expect(screen.getByTestId("conditions")).toBeEmptyDOMElement()

        settle(suggestions)

        await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"))
    })

    it("offers a condition pair per boolean attribute once ready", async () => {
        fetchPathSuggestions.mockResolvedValue(suggestions)

        renderProbe()

        await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"))

        expect(screen.getByTestId("conditions")).toHaveTextContent(
            "{% if user.data.isPro %}|{% unless user.data.isPro %}|" +
                "{% if user.data.admin %}|{% unless user.data.admin %}",
        )
    })

    // A project whose schema endpoints are down still needs a usable editor,
    // so readiness is about the fetch having settled, not having succeeded.
    it("becomes ready even when the schema fetch fails", async () => {
        vi.spyOn(console, "error").mockImplementation(() => {})
        fetchPathSuggestions.mockRejectedValue(new Error("schema endpoint down"))

        renderProbe()

        await waitFor(() => expect(screen.getByTestId("ready")).toHaveTextContent("true"))
        expect(screen.getByTestId("conditions")).toBeEmptyDOMElement()
    })
})
