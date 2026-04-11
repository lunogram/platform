import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import RuleOrganizationEventName from "./RuleOrganizationEventName"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Input } from "@/components/ui/input"
import type { OrganizationEventRule, OrganizationUserMatch } from "../../../types"
import {
    frequencyOperators,
    operatorTypes,
    periodUnits,
    userMatchTypes,
    createWrapperRule,
    createInitialMemberCondition,
} from "./RuleHelpers"
import MemberConditionsBuilder from "./MemberConditionsBuilder"

interface OrganizationEventRuleEditProps {
    rule: OrganizationEventRule
    eventName?: string
    setRule: (rule: OrganizationEventRule) => void
}

const DEFAULT_FREQUENCY = {
    period: {
        type: "rolling" as const,
        unit: "day" as const,
        value: 30,
    },
    operator: ">=" as const,
    count: 1,
}

export default function OrganizationEventRuleEdit({
    rule,
    setRule,
    eventName,
}: OrganizationEventRuleEditProps) {
    const { t } = useTranslation()

    const frequency = rule.frequency ?? DEFAULT_FREQUENCY

    const userMatch = rule.user_match ?? {
        type: "all" as const,
    }

    // Set default frequency and user_match if missing
    useEffect(() => {
        if (!rule.frequency || !rule.user_match) {
            setRule({
                ...rule,
                frequency: rule.frequency ?? DEFAULT_FREQUENCY,
                user_match: rule.user_match ?? { type: "all" },
            })
        }
    }, [rule, setRule])

    // When editing nested conditions, show a simpler header
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

    const handleUserMatchTypeChange = (type: OrganizationUserMatch["type"]) => {
        const newUserMatch: OrganizationUserMatch = { type }

        if (type === "conditions") {
            if (rule.user_match?.member_conditions) {
                newUserMatch.member_conditions = rule.user_match.member_conditions
            } else {
                const wrapper = createWrapperRule()
                wrapper.children = [createInitialMemberCondition(wrapper)]
                newUserMatch.member_conditions = wrapper
            }
        }

        setRule({
            ...rule,
            user_match: newUserMatch,
        })
    }

    return (
        <div className="w-full flex flex-col gap-2.5">
            <div className="flex items-center justify-start gap-1.5 flex-wrap text-sm">
                {t("rule_organization_did")}
                <RuleOrganizationEventName rule={rule} setRule={setRule} />
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
                        aria-label="Organization event frequency count"
                        className="h-8 w-16 rounded-none border-l-0 text-xs shadow-none"
                        value={frequency.count?.toString() ?? ""}
                        onChange={(e) => {
                            setRule({
                                ...rule,
                                frequency: {
                                    ...(rule.frequency ?? frequency),
                                    count: e.target.value
                                        ? parseInt(e.target.value, 10)
                                        : undefined,
                                },
                            })
                        }}
                    />
                    <span className="h-8 inline-flex items-center px-2 text-xs text-muted-foreground border border-l-0 bg-muted/50 rounded-r-md">
                        {t("rule_times", "times")}
                    </span>
                </div>
                {frequency.period.type === "rolling" && (
                    <div className="flex items-center">
                        <span className="h-8 inline-flex items-center px-2 text-xs text-muted-foreground border bg-muted/50 rounded-l-md">
                            {t("rule_in_last", "in last")}
                        </span>
                        <Input
                            type="text"
                            placeholder="Value"
                            aria-label="Organization event frequency value"
                            className="h-8 w-16 rounded-none border-l-0 text-xs shadow-none"
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

            {/* User matching section */}
            <div
                className="ml-5 p-3 bg-muted/50 rounded-lg border"
                role="group"
                aria-label={t("organization_event_user_match", "Organization event user match")}
            >
                <div className="flex items-center gap-1.5 text-sm">
                    {t("rule_include_org_members")}
                    <Select
                        value={userMatch.type}
                        onValueChange={(value) =>
                            handleUserMatchTypeChange(value as OrganizationUserMatch["type"])
                        }
                    >
                        <SelectTrigger
                            elevation="flat"
                            className="h-8 w-auto min-w-[180px] text-xs"
                            aria-label={t("organization_event_member_scope", "Organization event member scope")}
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            {userMatchTypes.map((mt) => (
                                <SelectItem key={mt.key} value={mt.key}>
                                    {mt.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>

                {/* Member conditions builder when conditions type is selected */}
                {userMatch.type === "conditions" && userMatch.member_conditions && (
                    <div className="mt-3 pt-3 border-t">
                        <MemberConditionsBuilder
                            rule={userMatch.member_conditions}
                            setRule={(member_conditions) =>
                                setRule({
                                    ...rule,
                                    user_match: {
                                        ...userMatch,
                                        member_conditions,
                                    },
                                })
                            }
                        />
                    </div>
                )}
            </div>

            {/* Event conditions section */}
            {!!rule.children?.length && (
                <div className="flex items-center gap-1.5 text-sm">
                    {t("rule_event_conditions_matching")}
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
