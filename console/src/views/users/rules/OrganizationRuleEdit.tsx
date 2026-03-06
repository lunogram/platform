import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import type { OrganizationRule, OrganizationUserMatch } from "../../../types"
import {
    operatorTypes,
    userMatchTypes,
    createWrapperRule,
    createInitialMemberCondition,
} from "./RuleHelpers"
import MemberConditionsBuilder from "./MemberConditionsBuilder"

interface OrganizationRuleEditProps {
    rule: OrganizationRule
    setRule: (rule: OrganizationRule) => void
    showUserMatch?: boolean
}

export default function OrganizationRuleEdit({
    rule,
    setRule,
    showUserMatch = false,
}: OrganizationRuleEditProps) {
    const { t } = useTranslation()

    const userMatch = rule.user_match ?? {
        type: "all",
    }

    // Set default user_match if missing
    useEffect(() => {
        if (!rule.user_match) {
            setRule({
                ...rule,
                user_match: { type: "all" },
            })
        }
    }, [rule, setRule])

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

    // When showUserMatch is false, render just the header
    if (!showUserMatch) {
        return (
            <div className="flex items-center gap-1.5">
                {t("rule_organization_has")}
                {!!rule.children?.length && (
                    <>
                        {t("rule_matching")}
                        <Select
                            value={rule.operator}
                            onValueChange={(operator) =>
                                setRule({ ...rule, operator: operator as typeof rule.operator })
                            }
                        >
                            <SelectTrigger className="h-8 w-auto min-w-[70px] text-xs shadow-none">
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
                )}
            </div>
        )
    }

    // When showUserMatch is true, render only the user match section
    return (
        <div className="ml-5 p-3 bg-muted/50 rounded-lg border">
            <div className="flex items-center gap-1.5 text-sm">
                {t("rule_include_org_members")}
                <Select
                    value={userMatch.type}
                    onValueChange={(value) =>
                        handleUserMatchTypeChange(value as OrganizationUserMatch["type"])
                    }
                >
                    <SelectTrigger className="h-8 w-auto min-w-[180px] text-xs shadow-none">
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
    )
}
