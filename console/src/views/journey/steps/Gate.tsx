import { useContext } from "react"
import type { JourneyStepType, Rule } from "../../../types"
import { GateStepIcon } from "../../../components/icons"
import RuleBuilder from "../../users/rules/RuleBuilder"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useTranslation } from "react-i18next"
import { ruleDescription } from "../../users/rules/RuleDescriptions"
import { createWrapperRule, isStepVisitRule, isWrapper } from "../../users/rules/RuleHelpers"
import type { JourneyStepOption } from "../JourneyVariableContext"
import { useJourneyVariableContext } from "../JourneyVariableContext"

interface GateConfig {
    rule: Rule
}

/**
 * Replaces the step id a step visit rule points at with the name shown on the
 * canvas, so the gate summary reads as the journey does.
 */
function withStepNames(rule: Rule, steps: JourneyStepOption[]): Rule {
    if (isStepVisitRule(rule)) {
        return { ...rule, path: steps.find(({ id }) => id === rule.path)?.label ?? rule.path }
    }

    if (isWrapper(rule)) {
        return { ...rule, children: rule.children.map((child) => withStepNames(child, steps)) }
    }

    return rule
}

export const gateStep: JourneyStepType<GateConfig> = {
    name: "gate",
    icon: <GateStepIcon />,
    category: "flow",
    description: "gate_desc",
    Describe({ value }) {
        const { t } = useTranslation()
        const [preferences] = useContext(PreferencesContext)
        const { steps } = useJourneyVariableContext()
        if (value.rule) {
            const rule = withStepNames(value.rule, steps)
            return (
                <div className="max-w-[300px]">
                    {t("has_done") + " "}
                    {ruleDescription(preferences, rule, [], rule.operator)}
                </div>
            )
        }
        return null
    },
    newData: async () => ({
        rule: createWrapperRule(),
    }),
    Edit({ onChange, value, nodeId }) {
        const { t } = useTranslation()
        const { getVariablesForNode, steps } = useJourneyVariableContext()
        const journeyVariables = nodeId ? getVariablesForNode(nodeId) : []
        return (
            <RuleBuilder
                rule={value.rule}
                setRule={(rule) => onChange({ ...value, rule })}
                headerPrefix={t("does_user_match")}
                userOnly={true}
                journeyContext={true}
                journeyVariables={journeyVariables}
                journeySteps={steps}
                currentStepId={nodeId}
            />
        )
    },
    sources: ["yes", "no"],
}
