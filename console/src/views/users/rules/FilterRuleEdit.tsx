import { useContext, useMemo } from "react"
import { highlightSearch } from "@/lib/ui-utils"
import type { RuleEditProps } from "./RuleHelpers"
import { operatorTypes, VariablesContext, ruleTypes } from "./RuleHelpers"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Combobox } from "../../../components/ui/combobox"
import { Input } from "@/components/ui/input"
import type { EventSchemaPath, OrganizationSchemaPath, UserSchemaPath } from "../../../types"

type PathOption = UserSchemaPath | EventSchemaPath | OrganizationSchemaPath

export default function FilterRuleEdit({
    rule,
    setRule,
    group,
    eventName = "",
    controls,
}: Omit<RuleEditProps, "root" | "headerPrefix" | "depth">) {
    const { suggestions } = useContext(VariablesContext)
    const { path } = rule ?? {}
    const hasValue = rule?.operator && !["is set", "is not set", "empty"].includes(rule?.operator)

    const isEventGroup = group === "event"
    const isOrganizationEventGroup = group === "organization_event"
    const isOrganizationGroup = group === "organization"

    const pathSuggestions = useMemo<PathOption[]>(() => {
        if (isEventGroup || isOrganizationEventGroup) {
            if (!eventName) return []
            const eventSource =
                isOrganizationEventGroup && suggestions.organizationEventPaths
                    ? suggestions.organizationEventPaths
                    : suggestions.eventPaths
            const event = eventSource.find((e) => e.name === eventName)
            if (!event) return []
            let schemaPaths = event.schema
            if (path) {
                const search = path.toLowerCase()
                schemaPaths = schemaPaths.filter((s) => s.path.toLowerCase().includes(search))
            }
            return schemaPaths
        }

        if (isOrganizationGroup) {
            let orgPaths = suggestions.organizationPaths ?? []
            if (path) {
                const search = path.toLowerCase()
                orgPaths = orgPaths.filter((p) => p.path.toLowerCase().includes(search))
            }
            return orgPaths
        }

        let paths = suggestions.userPaths
        if (path) {
            const search = path.toLowerCase()
            paths = paths.filter((p) => p.path.toLowerCase().includes(search))
        }
        return paths
    }, [suggestions, isEventGroup, isOrganizationEventGroup, isOrganizationGroup, eventName, path])

    const getOptionDataType = (option: PathOption): string => {
        return option.types[0] || "string"
    }

    return (
        <div className="relative flex items-start gap-2.5 -ml-px pl-5 py-1.5 border-l border-border last:border-l-transparent after:content-[''] after:absolute after:left-[-1px] after:top-0 after:w-5 after:h-5 after:border-b after:border-l after:border-border after:rounded-bl-md">
            <div className="flex items-center">
                <Select
                    value={rule?.type}
                    onValueChange={(type) => setRule({ ...rule, type: type as typeof rule.type })}
                >
                    <SelectTrigger className="h-8 w-auto min-w-[90px] rounded-r-none text-xs shadow-none">
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
                        const suggestion = pathSuggestions.find((s) => s.path === selectedPath)
                        if (suggestion) {
                            setRule({
                                ...rule,
                                type: getOptionDataType(suggestion) as typeof rule.type,
                                path: suggestion.path,
                            })
                        } else {
                            setRule({ ...rule, path: selectedPath })
                        }
                    }}
                    options={pathSuggestions}
                    placeholder="Path"
                    required
                    inputClassName="rounded-none border-l-0"
                    buttonClassName="rounded-none"
                    renderOption={(option, search) => (
                        <span
                            dangerouslySetInnerHTML={{
                                __html: highlightSearch(option.path, search),
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
                    <SelectTrigger className="h-8 w-auto min-w-[100px] rounded-none border-l-0 text-xs shadow-none">
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
                        <SelectTrigger className="h-8 w-auto min-w-[80px] rounded-none border-l-0 text-xs shadow-none">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="true">True</SelectItem>
                            <SelectItem value="false">False</SelectItem>
                        </SelectContent>
                    </Select>
                ) : (
                    <Input
                        type="text"
                        placeholder="Value"
                        className="h-8 min-w-[100px] w-auto rounded-none border-l-0 text-xs shadow-none"
                        value={rule?.value?.toString() ?? ""}
                        onChange={(e) => setRule({ ...rule, value: e.target.value })}
                    />
                )}
                {controls}
            </div>
        </div>
    )
}
