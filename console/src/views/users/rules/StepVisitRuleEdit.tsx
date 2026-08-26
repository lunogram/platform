import { useTranslation } from "react-i18next"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Input } from "@/components/ui/input"
import type { StepVisitRule } from "../../../types"
import type { RuleEditProps } from "./RuleHelpers"
import { stepVisitOperators, stepVisitScopes } from "./RuleHelpers"

const CURRENT_STEP = "__current__"

/**
 * StepVisitRuleEdit renders a leaf rule editor comparing how often a user
 * reached a journey step, which is what limits a recursive flow to a set
 * number of passes. An empty path means the step the gate itself sits on.
 */
export default function StepVisitRuleEdit({
    rule,
    setRule,
    controls,
    journeySteps,
    currentStepId,
}: Omit<RuleEditProps<StepVisitRule>, "root" | "headerPrefix" | "depth">) {
    const { t } = useTranslation()

    const otherSteps = (journeySteps ?? []).filter(({ id }) => id !== currentStepId)

    return (
        <div className="relative flex items-start gap-2.5 -ml-px pl-5 py-1.5 border-l border-border last:border-l-transparent after:content-[''] after:absolute after:left-[-1px] after:top-0 after:w-5 after:h-5 after:border-b after:border-l after:border-border after:rounded-bl-md">
            <div className="flex flex-wrap items-center gap-1.5 gap-y-1.5 text-sm">
                {t("rule_step_visit_has_passed")}
                <Select
                    value={rule?.path ? rule.path : CURRENT_STEP}
                    onValueChange={(path) =>
                        setRule({ ...rule, path: path === CURRENT_STEP ? "" : path })
                    }
                >
                    <SelectTrigger elevation="flat" className="h-8 w-auto min-w-[120px] text-xs">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value={CURRENT_STEP}>
                            {t("rule_step_visit_this_step")}
                        </SelectItem>
                        {otherSteps.map((step) => (
                            <SelectItem key={step.id} value={step.id}>
                                {step.label}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                <div className="flex items-center">
                    <Select
                        value={rule?.operator}
                        onValueChange={(operator) =>
                            setRule({ ...rule, operator: operator as typeof rule.operator })
                        }
                    >
                        <SelectTrigger
                            elevation="flat"
                            className="h-8 w-auto min-w-[100px] rounded-r-none text-xs"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {stepVisitOperators.map((op) => (
                                <SelectItem key={op.key} value={op.key}>
                                    {op.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <Input
                        type="text"
                        placeholder="Count"
                        aria-label="Step visit count"
                        className="h-8 w-16 rounded-none border-l-0 text-xs shadow-none"
                        value={rule?.value ?? ""}
                        onChange={(e) =>
                            setRule({ ...rule, value: e.target.value.replace(/[^0-9]/g, "") })
                        }
                    />
                    <span
                        className="h-8 inline-flex items-center px-2 text-xs text-muted-foreground border border-l-0 bg-muted/50"
                        title={t("rule_step_visit_hint")}
                    >
                        {t("rule_step_visit_times")}
                    </span>
                    <Select
                        value={rule?.step_scope ?? "entry"}
                        onValueChange={(scope) =>
                            setRule({ ...rule, step_scope: scope as typeof rule.step_scope })
                        }
                    >
                        <SelectTrigger
                            elevation="flat"
                            className="h-8 w-auto min-w-[150px] rounded-l-none border-l-0 text-xs"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {stepVisitScopes.map((scope) => (
                                <SelectItem key={scope.key} value={scope.key}>
                                    {scope.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    {controls}
                </div>
            </div>
        </div>
    )
}
