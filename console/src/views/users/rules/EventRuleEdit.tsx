import { useTranslation } from "react-i18next"
import RuleEventName from "./RuleEventName"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Input } from "@/components/ui/input"
import type { EventRule } from "../../../types"
import { frequencyOperators, operatorTypes, periodUnits } from "./RuleHelpers"

interface EventRuleEditProps {
    rule: EventRule
    eventName?: string
    setRule: (rule: EventRule) => void
    journeyContext?: boolean
}

export default function EventRuleEdit({
    rule,
    setRule,
    eventName,
    journeyContext = false,
}: EventRuleEditProps) {
    const { t } = useTranslation()

    if (eventName) {
        if (rule.children?.length) {
            return (
                <>
                    {t("rule_matching")}
                    <Select
                        value={rule.operator}
                        onValueChange={(operator) =>
                            setRule({ ...rule, operator: operator as typeof rule.operator })
                        }
                    >
                        <SelectTrigger elevation="flat" className="h-8 w-auto min-w-[70px] text-xs">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {operatorTypes.wrapper.map((op) => (
                                <SelectItem key={op.key} value={op.key}>
                                    {op.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    {t("rule_of_the_following")}
                </>
            )
        }
        return <></>
    }

    const frequency = rule.frequency ?? {
        period: {
            type: "rolling" as const,
            unit: "day" as const,
            value: 30,
        },
        operator: ">=" as const,
        count: 1,
    }

    // If missing frequency, set default values
    if (!rule.frequency) {
        setRule({
            ...rule,
            frequency,
        })
    }

    return (
        <div className="w-full flex flex-col gap-2.5">
            <div className="flex items-center justify-start gap-1.5 flex-wrap text-sm">
                {t("rule_did")}
                <RuleEventName rule={rule} setRule={setRule} />
                <div className="flex items-center">
                    <Select
                        value={frequency.operator}
                        onValueChange={(operator) =>
                            setRule({
                                ...rule,
                                frequency: {
                                    ...(rule.frequency ?? frequency),
                                    operator: operator as typeof frequency.operator,
                                },
                            })
                        }
                    >
                        <SelectTrigger
                            elevation="flat"
                            className="h-8 w-auto min-w-[80px] rounded-r-none text-xs"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {frequencyOperators.map((op) => (
                                <SelectItem key={op.key} value={op.key}>
                                    {op.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <Input
                        type="text"
                        placeholder="Count"
                        aria-label="Event frequency count"
                        className="h-8 w-16 rounded-none border-l-0 text-xs shadow-none"
                        value={frequency.count?.toString() ?? ""}
                        onChange={(e) => {
                            const count = e.target.value ? parseInt(e.target.value, 10) : undefined
                            setRule({
                                ...rule,
                                frequency: {
                                    ...(rule.frequency ?? frequency),
                                    count,
                                },
                            })
                        }}
                    />
                    <span className="h-8 inline-flex items-center px-2 text-xs text-muted-foreground border border-l-0 bg-muted/50 rounded-r-md">
                        {"times"}
                    </span>
                </div>
                {journeyContext && (
                    <Select
                        value={frequency.period.type}
                        onValueChange={(periodType) => {
                            if (periodType === "since_entered") {
                                setRule({
                                    ...rule,
                                    frequency: {
                                        ...frequency,
                                        period: { type: "since_entered" },
                                    },
                                })
                            } else {
                                setRule({
                                    ...rule,
                                    frequency: {
                                        ...frequency,
                                        period: {
                                            type: "rolling",
                                            unit: "day",
                                            value: 30,
                                        },
                                    },
                                })
                            }
                        }}
                    >
                        <SelectTrigger
                            elevation="flat"
                            className="h-8 w-auto min-w-[160px] text-xs"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="rolling">in last</SelectItem>
                            <SelectItem value="since_entered">since entered journey</SelectItem>
                        </SelectContent>
                    </Select>
                )}
                {frequency.period.type === "rolling" && (
                    <div className="flex items-center">
                        {!journeyContext && (
                            <span className="h-8 inline-flex items-center px-2 text-xs text-muted-foreground border bg-muted/50 rounded-l-md">
                                {"in last"}
                            </span>
                        )}
                        <Input
                            type="text"
                            placeholder="Value"
                            aria-label="Event frequency value"
                            className={`h-8 w-16 rounded-none ${journeyContext ? "rounded-l-md" : "border-l-0"} text-xs shadow-none`}
                            value={frequency.period.value.toString()}
                            onChange={(e) => {
                                if (frequency.period.type !== "rolling") return
                                setRule({
                                    ...rule,
                                    frequency: {
                                        ...frequency,
                                        period: {
                                            ...frequency.period,
                                            value: parseInt(e.target.value, 10) || 1,
                                        },
                                    },
                                })
                            }}
                        />
                        <Select
                            value={frequency.period.unit}
                            onValueChange={(unit) => {
                                if (frequency.period.type !== "rolling") return
                                setRule({
                                    ...rule,
                                    frequency: {
                                        ...frequency,
                                        period: {
                                            ...frequency.period,
                                            unit: unit as typeof frequency.period.unit,
                                        },
                                    },
                                })
                            }}
                        >
                            <SelectTrigger
                                elevation="flat"
                                className="h-8 w-auto min-w-[80px] rounded-l-none border-l-0 text-xs"
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {periodUnits.map((u) => (
                                    <SelectItem key={u.key} value={u.key}>
                                        {u.label}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                )}
            </div>
            {!!rule.children?.length && (
                <div className="flex items-center gap-1.5 text-sm">
                    {t("rule_matching")}
                    <Select
                        value={rule.operator}
                        onValueChange={(operator) =>
                            setRule({ ...rule, operator: operator as typeof rule.operator })
                        }
                    >
                        <SelectTrigger elevation="flat" className="h-8 w-auto min-w-[70px] text-xs">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {operatorTypes.wrapper.map((op) => (
                                <SelectItem key={op.key} value={op.key}>
                                    {op.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    {t("rule_of_the_following")}
                </div>
            )}
        </div>
    )
}
