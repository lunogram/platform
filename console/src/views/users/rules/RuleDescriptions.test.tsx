import { describe, expect, it } from "vitest"
import { render } from "@testing-library/react"
import type { Preferences, Rule, StepVisitRule } from "@/types"
import { ruleDescription } from "./RuleDescriptions"
import { createStepVisitRule } from "./RuleHelpers"
import { createUuid } from "@/utils"

const preferences: Preferences = { lang: "en", mode: "light", timeZone: "UTC" }

const describeRule = (rule: Rule) =>
    render(<>{ruleDescription(preferences, rule, [], rule.operator)}</>).container.textContent

describe("step visit descriptions", () => {
    it("describes a rule on the step it sits on", () => {
        const rule: StepVisitRule = {
            ...createStepVisitRule(),
            operator: ">",
            value: "3",
        }

        expect(describeRule(rule)).toBe("this step more than 3 times in this journey run")
    })

    it("describes a rule on another step across all runs", () => {
        const rule: StepVisitRule = {
            ...createStepVisitRule(),
            path: "Reminder email",
            operator: "<=",
            value: "2",
            step_scope: "journey",
        }

        expect(describeRule(rule)).toBe("Reminder email at most 2 times across all runs")
    })

    it("describes a step visit rule nested in a wrapper", () => {
        const wrapper: Rule = {
            uuid: createUuid(),
            path: "",
            type: "wrapper",
            group: "parent",
            operator: "and",
            children: [{ ...createStepVisitRule(), operator: ">", value: "1" }],
        }

        expect(describeRule(wrapper)).toBe("this step more than 1 times in this journey run")
    })

    it("keeps step visit rules on the same step apart", () => {
        const wrapper: Rule = {
            uuid: createUuid(),
            path: "",
            type: "wrapper",
            group: "parent",
            operator: "or",
            children: [
                { ...createStepVisitRule(), operator: ">", value: "3" },
                { ...createStepVisitRule(), operator: ">", value: "10", step_scope: "journey" },
            ],
        }

        expect(describeRule(wrapper)).toBe(
            "this step more than 3 times in this journey run, or this step more than 10 times across all runs",
        )
    })
})
