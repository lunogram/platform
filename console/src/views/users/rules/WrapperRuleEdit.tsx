import type { ReactElement } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Plus, Trash2 } from "lucide-react"
import { createUuid } from "../../../utils"
import EventRuleEdit from "./EventRuleEdit"
import OrganizationEventRuleEdit from "./OrganizationEventRuleEdit"
import OrganizationRuleEdit from "./OrganizationRuleEdit"
import type { RuleEditProps } from "./RuleHelpers"
import {
    createEventRule,
    createOrganizationEventRule,
    createOrganizationRule,
    isEventWrapper,
    isOrganizationEventWrapper,
    isOrganizationWrapper,
    operatorTypes,
} from "./RuleHelpers"
import type {
    EventRule,
    OrganizationEventRule,
    OrganizationRule,
    WrapperRule,
} from "../../../types"
import RuleEdit from "./RuleEdit"

export default function WrapperRuleEdit({
    rule,
    root,
    setRule,
    controls,
    depth = 0,
    eventName = "",
    userOnly = false,
    journeyContext = false,
    journeyVariables,
}: RuleEditProps<WrapperRule>) {
    const { t } = useTranslation()

    const handleAddEventWrapper = () => {
        const children = rule?.children ?? []
        const newRule: EventRule = createEventRule(rule)
        setRule({
            ...rule,
            children: [...children, newRule],
        })
    }

    const handleAddOrganizationEventWrapper = () => {
        const children = rule?.children ?? []
        const newRule: OrganizationEventRule = createOrganizationEventRule(rule)
        setRule({
            ...rule,
            children: [...children, newRule],
        })
    }

    const handleAddOrganizationWrapper = () => {
        const children = rule?.children ?? []
        const newRule: OrganizationRule = createOrganizationRule(rule)
        setRule({
            ...rule,
            children: [...children, newRule],
        })
    }

    let ruleSet = (
        <div className="w-full flex flex-col items-start pb-2.5 rounded-md">
            {/* Header */}
            <div
                className={`w-full flex justify-start gap-1.5 text-sm ${isOrganizationEventWrapper(rule) || isOrganizationWrapper(rule) || isEventWrapper(rule) ? "items-start" : "items-center"}`}
            >
                {isOrganizationWrapper(rule) ? (
                    <OrganizationRuleEdit rule={rule} setRule={setRule} showUserMatch={false} />
                ) : isOrganizationEventWrapper(rule) ? (
                    <OrganizationEventRuleEdit
                        rule={rule}
                        setRule={setRule}
                        eventName={eventName}
                    />
                ) : isEventWrapper(rule) ? (
                    <EventRuleEdit
                        rule={rule}
                        setRule={setRule}
                        eventName={eventName}
                        journeyContext={journeyContext}
                    />
                ) : (
                    <>
                        {t("rule_include_users_matching")}
                        <Select
                            value={rule?.operator}
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
                <div className="flex-1" />
                {(isEventWrapper(rule) ||
                    isOrganizationEventWrapper(rule) ||
                    isOrganizationWrapper(rule)) &&
                    controls &&
                    typeof controls === "object" &&
                    "props" in controls && (
                        <Button
                            size="sm"
                            variant="outline"
                            className="h-8 shrink-0 shadow-none"
                            onClick={
                                (controls as ReactElement<{ onClick?: () => void }>).props.onClick
                            }
                        >
                            <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                    )}
                {!isEventWrapper(rule) &&
                    !isOrganizationEventWrapper(rule) &&
                    !isOrganizationWrapper(rule) &&
                    controls}
            </div>

            {/* User match section for organization rules - rendered before children */}
            {isOrganizationWrapper(rule) && (
                <OrganizationRuleEdit rule={rule} setRule={setRule} showUserMatch={true} />
            )}

            {/* Children rules */}
            <div className="flex flex-col py-1 ml-2.5">
                {rule?.children?.map((child, index, arr) => (
                    <RuleEdit
                        key={index}
                        root={root}
                        rule={child}
                        setRule={(child) =>
                            setRule({
                                ...rule,
                                children: rule?.children?.map((c, i) => (i === index ? child : c)),
                            })
                        }
                        group={rule?.group}
                        eventName={rule?.value}
                        depth={depth + 1}
                        userOnly={userOnly}
                        journeyContext={journeyContext}
                        journeyVariables={journeyVariables}
                        controls={
                            <Button
                                size="sm"
                                variant="outline"
                                className="h-8 rounded-l-none shadow-none border-l-0"
                                onClick={() =>
                                    setRule({
                                        ...rule,
                                        children: arr.filter((_, i) => i !== index),
                                    })
                                }
                            >
                                <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                        }
                    />
                ))}
            </div>

            {/* Add Property Condition button for organization rules - rendered after children */}
            {isOrganizationWrapper(rule) && (
                <div className="flex gap-1.5 px-5 mt-2">
                    <Button
                        size="sm"
                        variant="outline"
                        className="shadow-none"
                        aria-label="Add condition"
                        onClick={() => {
                            setRule({
                                ...rule,
                                children: [
                                    ...(rule?.children ?? []),
                                    {
                                        uuid: createUuid(),
                                        root_uuid: root?.uuid,
                                        parent_uuid: rule?.uuid,
                                        path: "",
                                        type: "string",
                                        group: "organization" as const,
                                        value: "",
                                        operator: "=",
                                    },
                                ],
                            })
                        }}
                    >
                        <Plus className="h-3.5 w-3.5 mr-1" />
                        {t("rule_add_org_property_condition")}
                    </Button>
                </div>
            )}

            {/* Action buttons (for non-organization rules) */}
            {!isOrganizationWrapper(rule) && (
                <div className={`flex flex-wrap gap-1.5 px-5 mt-2${depth === 0 ? " ml-2.5" : ""}`}>
                    <Button
                        size="sm"
                        variant="outline"
                        className="shadow-none"
                        onClick={() => {
                            let childGroup:
                                | "user"
                                | "event"
                                | "organization_event"
                                | "organization"
                                | "journey" = "user"
                            if (rule?.group === "event" || rule?.group === "organization_event") {
                                childGroup = rule.group
                            }

                            setRule({
                                ...rule,
                                children: [
                                    ...(rule?.children ?? []),
                                    {
                                        uuid: createUuid(),
                                        root_uuid: root?.uuid,
                                        parent_uuid: rule?.uuid,
                                        path: "",
                                        type: "string",
                                        group: childGroup,
                                        value: "",
                                        operator: "=",
                                    },
                                ],
                            })
                        }}
                    >
                        <Plus className="h-3.5 w-3.5 mr-1" />
                        {rule?.group === "event" || rule?.group === "organization_event"
                            ? t("rule_add_condition")
                            : t("rule_add_user_condition")}
                    </Button>
                    {depth === 0 && (rule?.group === "user" || rule?.group === "parent") && (
                        <>
                            <Button
                                size="sm"
                                variant="outline"
                                className="shadow-none"
                                aria-label="Add event condition"
                                onClick={() => handleAddEventWrapper()}
                            >
                                <Plus className="h-3.5 w-3.5 mr-1" />
                                {t("rule_add_event_condition")}
                            </Button>
                            {!userOnly && (
                                <>
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        className="shadow-none"
                                        aria-label="Add organization event condition"
                                        onClick={() => handleAddOrganizationEventWrapper()}
                                    >
                                        <Plus className="h-3.5 w-3.5 mr-1" />
                                        {t("rule_add_org_event_condition")}
                                    </Button>
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        className="shadow-none"
                                        aria-label="Add organization condition"
                                        onClick={() => handleAddOrganizationWrapper()}
                                    >
                                        <Plus className="h-3.5 w-3.5 mr-1" />
                                        {t("rule_add_org_condition")}
                                    </Button>
                                </>
                            )}
                            {journeyContext && journeyVariables && journeyVariables.length > 0 && (
                                <Button
                                    size="sm"
                                    variant="outline"
                                    className="shadow-none"
                                    onClick={() => {
                                        setRule({
                                            ...rule,
                                            children: [
                                                ...(rule?.children ?? []),
                                                {
                                                    uuid: createUuid(),
                                                    root_uuid: root?.uuid,
                                                    parent_uuid: rule?.uuid,
                                                    path: "",
                                                    type: "string",
                                                    group: "journey" as const,
                                                    value: "",
                                                    operator: "=",
                                                },
                                            ],
                                        })
                                    }}
                                >
                                    <Plus className="h-3.5 w-3.5 mr-1" />
                                    {t("rule_add_journey_condition")}
                                </Button>
                            )}
                        </>
                    )}
                </div>
            )}
        </div>
    )

    if (depth > 0) {
        ruleSet = (
            <div className="relative flex items-start gap-2.5 -ml-px pl-5 py-1.5 border-l border-border last:border-l-transparent after:content-[''] after:absolute after:left-[-1px] after:top-0 after:w-5 after:h-5 after:border-b after:border-l after:border-border after:rounded-bl-md">
                <div className="w-full border rounded-md p-2.5">{ruleSet}</div>
            </div>
        )
    }

    return ruleSet
}
