import { describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { StepVisitRule } from "@/types"
import StepVisitRuleEdit from "./StepVisitRuleEdit"
import { createStepVisitRule } from "./RuleHelpers"

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: (key: string) => key }),
}))

const steps = [
    { id: "gate", label: "Limit", type: "gate" },
    { id: "reminder", label: "Reminder email", type: "campaign" },
]

const renderRule = (rule: StepVisitRule, setRule = vi.fn()) => {
    render(
        <StepVisitRuleEdit
            rule={rule}
            setRule={setRule}
            group="parent"
            journeySteps={steps}
            currentStepId="gate"
        />,
    )
    return setRule
}

describe("StepVisitRuleEdit", () => {
    it("defaults to the step it sits on, counted within the current run", () => {
        renderRule(createStepVisitRule())

        expect(screen.getByText("rule_step_visit_this_step")).toBeInTheDocument()
        expect(screen.getByText("more than")).toBeInTheDocument()
        expect(screen.getByText("in this journey run")).toBeInTheDocument()
        expect(screen.getByLabelText("Step visit count")).toHaveValue("1")
    })

    it("shows the name of a referenced step", () => {
        renderRule({ ...createStepVisitRule(), path: "reminder" })

        expect(screen.getByText("Reminder email")).toBeInTheDocument()
    })

    it("keeps the count numeric", async () => {
        const setRule = renderRule(createStepVisitRule())

        await userEvent.type(screen.getByLabelText("Step visit count"), "2x")

        expect(setRule).toHaveBeenCalledWith(expect.objectContaining({ value: "12" }))
        expect(setRule).not.toHaveBeenCalledWith(expect.objectContaining({ value: "1x" }))
    })

    it("drops the count when the field is cleared", async () => {
        const setRule = renderRule(createStepVisitRule())

        await userEvent.clear(screen.getByLabelText("Step visit count"))

        expect(setRule).toHaveBeenCalledWith(expect.objectContaining({ value: undefined }))
    })
})
