import { useMemo } from "react"
import { highlightSearch } from "@/lib/ui-utils"
import type { RuleEditProps } from "./RuleHelpers"
import { operatorTypes, ruleTypes } from "./RuleHelpers"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Combobox } from "../../../components/ui/combobox"
import { Input } from "@/components/ui/input"
import { TemplateInput } from "@/components/ui/template-input"

interface PathOption {
    path: string
    types: string[]
}

/**
 * JourneyRuleEdit renders a leaf rule editor for journey data conditions.
 * It uses journey variables (from upstream steps) as path suggestions,
 * allowing users to build conditions like "journey.entrance.amount > 100".
 */
export default function JourneyRuleEdit({
    rule,
    setRule,
    controls,
    journeyVariables,
}: Omit<RuleEditProps, "root" | "headerPrefix" | "depth">) {
    const { path } = rule ?? {}
    const hasValue = rule?.operator && !["is set", "is not set", "empty"].includes(rule?.operator)

    // Flatten all journey variable groups into path options
    const pathSuggestions = useMemo<PathOption[]>(() => {
        if (!journeyVariables?.length) return []

        const allVars: PathOption[] = []
        for (const group of journeyVariables) {
            for (const v of group.variables) {
                // Skip the "full object" entries — they aren't useful as condition paths
                if (v.types?.includes("object") && v.types.length === 1) continue
                allVars.push({
                    path: v.path,
                    types: v.types ?? ["string"],
                })
            }
        }

        if (path) {
            const search = path.toLowerCase()
            return allVars.filter((v) => v.path.toLowerCase().includes(search))
        }

        return allVars
    }, [journeyVariables, path])

    const getOptionDataType = (option: PathOption): string => {
        return option.types[0] || "string"
    }

    return (
        <div className="relative flex items-start gap-2.5 -ml-px pl-5 py-1.5 border-l border-border last:border-l-transparent after:content-[''] after:absolute after:left-[-1px] after:top-0 after:w-5 after:h-5 after:border-b after:border-l after:border-border after:rounded-bl-md">
            <div className="flex flex-wrap items-center gap-y-1.5">
                <div className="flex items-center">
                    <Select
                        value={rule?.type}
                        onValueChange={(type) =>
                            setRule({ ...rule, type: type as typeof rule.type })
                        }
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
                        placeholder="Journey path"
                        required
                        inputClassName="rounded-none border-l-0"
                        buttonClassName="rounded-none"
                        renderOption={(option, search) => (
                            <span>{highlightSearch(option.path, search)}</span>
                        )}
                    />
                </div>
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
                                className="h-8 w-auto min-w-[80px] rounded-l-none border-l-0 text-xs"
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="true">True</SelectItem>
                                <SelectItem value="false">False</SelectItem>
                            </SelectContent>
                        </Select>
                    ) : journeyVariables?.length ? (
                        <TemplateInput
                            placeholder="Value"
                            className="h-8 min-w-[100px] w-auto rounded-l-none border-l-0 text-xs shadow-none"
                            value={rule?.value?.toString() ?? ""}
                            onChange={(val) => setRule({ ...rule, value: val })}
                            variables={journeyVariables}
                            variant="compact"
                        />
                    ) : (
                        <Input
                            type="text"
                            placeholder="Value"
                            className="h-8 min-w-[100px] w-auto rounded-l-none border-l-0 text-xs shadow-none"
                            value={rule?.value?.toString() ?? ""}
                            onChange={(e) => setRule({ ...rule, value: e.target.value })}
                        />
                    )}
                    {controls}
                </div>
            </div>
        </div>
    )
}
