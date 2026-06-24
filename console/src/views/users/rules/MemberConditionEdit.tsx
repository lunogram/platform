import { Button } from "@/components/ui/button"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Combobox } from "../../../components/ui/combobox"
import { Input } from "@/components/ui/input"
import { Trash2 } from "lucide-react"
import type { OrganizationUserSchemaPath, Rule, RuleType } from "../../../types"
import { operatorTypes, ruleTypes } from "./RuleHelpers"
import { highlightSearch } from "@/lib/ui-utils"

interface MemberConditionEditProps {
    rule: Rule
    setRule: (rule: Rule) => void
    onRemove: () => void
    organizationUserPaths: OrganizationUserSchemaPath[]
}

// Map schema types to rule types
function mapSchemaTypeToRuleType(types: string[]): RuleType {
    const type = types[0]?.toLowerCase()
    switch (type) {
        case "string":
            return "string"
        case "number":
        case "integer":
        case "float":
            return "number"
        case "boolean":
        case "bool":
            return "boolean"
        case "date":
        case "datetime":
        case "timestamp":
            return "date"
        case "array":
        case "list":
            return "array"
        default:
            return "string"
    }
}

/**
 * Edit a single member property condition.
 * This is for filtering organization members based on their membership data properties.
 */
export default function MemberConditionEdit({
    rule,
    setRule,
    onRemove,
    organizationUserPaths,
}: MemberConditionEditProps) {
    const hasValue = rule?.operator && !["is set", "is not set", "empty"].includes(rule?.operator)

    // Convert schema paths to combobox options
    const pathOptions = organizationUserPaths.map((p) => {
        const dataType = mapSchemaTypeToRuleType(p.types)
        const name = p.path.startsWith(".") ? p.path.substring(1) : p.path
        return {
            id: `member-path-${p.path}`,
            path: p.path,
            name,
            type: "user" as const,
            data_type: dataType,
            visibility: "public" as const,
        }
    })

    return (
        <div className="flex items-center">
            <div className="flex items-center">
                <Select
                    value={rule?.type}
                    onValueChange={(type) => setRule({ ...rule, type: type as typeof rule.type })}
                >
                    <SelectTrigger
                        elevation="flat"
                        className="h-8 w-auto min-w-[90px] rounded-r-none text-xs"
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {ruleTypes.map((t) => (
                            <SelectItem key={t.key} value={t.key}>
                                {t.label}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                <Combobox
                    value={rule?.path}
                    onValueChange={(selectedPath: string) => {
                        const suggestion = pathOptions.find((s) => s.path === selectedPath)
                        if (suggestion) {
                            setRule({
                                ...rule,
                                type: suggestion.data_type as typeof rule.type,
                                path: suggestion.path,
                            })
                        } else {
                            setRule({ ...rule, path: selectedPath })
                        }
                    }}
                    options={pathOptions}
                    placeholder="Member property path"
                    ariaLabel="Member condition path"
                    inputAriaLabel="Member condition path"
                    required
                    inputClassName="rounded-none border-l-0"
                    buttonClassName="rounded-none"
                    renderOption={(option, search) => (
                        <span
                            dangerouslySetInnerHTML={{
                                __html: highlightSearch(option.name || option.path, search),
                            }}
                        />
                    )}
                />
                <Select
                    value={rule?.operator}
                    onValueChange={(operator) =>
                        setRule({ ...rule, operator: operator as typeof rule.operator })
                    }
                >
                    <SelectTrigger
                        elevation="flat"
                        className="h-8 w-auto min-w-[100px] rounded-none border-l-0 text-xs"
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {(operatorTypes[rule?.type] ?? []).map((op) => (
                            <SelectItem key={op.key} value={op.key}>
                                {op.label}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                {hasValue && rule.type === "boolean" ? (
                    <Select
                        value={
                            rule.value === "true"
                                ? "true"
                                : rule.value === "false"
                                  ? "false"
                                  : undefined
                        }
                        onValueChange={(value) => setRule({ ...rule, value })}
                    >
                        <SelectTrigger
                            elevation="flat"
                            className="h-8 w-auto min-w-[80px] rounded-none border-l-0 text-xs"
                        >
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="true">True</SelectItem>
                            <SelectItem value="false">False</SelectItem>
                        </SelectContent>
                    </Select>
                ) : (
                    hasValue && (
                        <Input
                            type="text"
                            placeholder="Value"
                            id="member-condition-value"
                            aria-label="Member condition value"
                            className="h-8 min-w-[100px] w-auto rounded-none border-l-0 text-xs shadow-none"
                            value={rule?.value?.toString() ?? ""}
                            onChange={(e) => setRule({ ...rule, value: e.target.value })}
                        />
                    )
                )}
                <Button
                    size="sm"
                    variant="outline"
                    className="h-8 rounded-l-none shadow-none border-l-0"
                    onClick={onRemove}
                >
                    <Trash2 className="h-3.5 w-3.5" />
                </Button>
            </div>
        </div>
    )
}
