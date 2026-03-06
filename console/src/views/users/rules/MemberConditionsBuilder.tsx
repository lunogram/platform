import { useContext } from "react"
import { Button } from "@/components/ui/button"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Plus } from "lucide-react"
import { createUuid } from "../../../utils"
import type { Rule, WrapperRule } from "../../../types"
import { operatorTypes, VariablesContext } from "./RuleHelpers"
import MemberConditionEdit from "./MemberConditionEdit"
import { useTranslation } from "react-i18next"

interface MemberConditionsBuilderProps {
    rule: WrapperRule
    setRule: (rule: WrapperRule) => void
}

/**
 * A simplified rule builder specifically for organization member property conditions.
 * This only supports property-based conditions on member data, not events.
 */
export default function MemberConditionsBuilder({ rule, setRule }: MemberConditionsBuilderProps) {
    const { t } = useTranslation()
    const { suggestions } = useContext(VariablesContext)

    const handleAddCondition = () => {
        const newCondition: Rule = {
            uuid: createUuid(),
            root_uuid: rule.uuid,
            parent_uuid: rule.uuid,
            path: "",
            type: "string",
            group: "user",
            value: "",
            operator: "=",
        }

        setRule({
            ...rule,
            children: [...(rule.children ?? []), newCondition],
        })
    }

    const handleRemoveCondition = (index: number) => {
        setRule({
            ...rule,
            children: rule.children?.filter((_, i) => i !== index) ?? [],
        })
    }

    const handleUpdateCondition = (index: number, updatedCondition: Rule) => {
        setRule({
            ...rule,
            children: rule.children?.map((c, i) => (i === index ? updatedCondition : c)) ?? [],
        })
    }

    return (
        <div className="flex flex-col gap-2.5">
            <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
                {t("rule_members_matching")}
                <Select
                    value={rule.operator}
                    onValueChange={(operator) =>
                        setRule({ ...rule, operator: operator as typeof rule.operator })
                    }
                >
                    <SelectTrigger className="h-8 w-auto min-w-[70px] text-xs">
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

            <div className="flex flex-col gap-1.5">
                {rule.children?.map((child, index) => (
                    <MemberConditionEdit
                        key={child.uuid || index}
                        rule={child}
                        setRule={(updated) => handleUpdateCondition(index, updated)}
                        onRemove={() => handleRemoveCondition(index)}
                        organizationUserPaths={suggestions.organizationUserPaths ?? []}
                    />
                ))}
            </div>

            <div>
                <Button size="sm" variant="outline" onClick={handleAddCondition}>
                    <Plus className="h-3.5 w-3.5 mr-1" />
                    {t("rule_add_member_condition")}
                </Button>
            </div>
        </div>
    )
}
