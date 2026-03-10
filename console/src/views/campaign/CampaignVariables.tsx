import { useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Braces, Plus, Trash2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import type { CampaignVariable } from "@/types"

interface CampaignVariablesProps {
    variables: CampaignVariable[]
    onChange: (variables: CampaignVariable[]) => void
}

const VARIABLE_NAME_REGEX = /^[a-z][a-z0-9_]*$/
const RESERVED_NAMES = ["user"]

function validateName(
    name: string,
    variables: CampaignVariable[],
    index: number,
): string | undefined {
    if (!name) return undefined // allow empty while typing
    if (!VARIABLE_NAME_REGEX.test(name))
        return "Must be lowercase letters, numbers, and underscores"
    if (RESERVED_NAMES.includes(name)) return `"${name}" is reserved`
    const duplicate = variables.findIndex((v, i) => i !== index && v.name === name)
    if (duplicate !== -1) return "Duplicate name"
    return undefined
}

export function CampaignVariables({ variables, onChange }: CampaignVariablesProps) {
    const { t } = useTranslation()

    const addVariable = useCallback(() => {
        onChange([...variables, { name: "" }])
    }, [variables, onChange])

    const updateVariable = useCallback(
        (index: number, updates: Partial<CampaignVariable>) => {
            const updated = variables.map((v, i) => (i === index ? { ...v, ...updates } : v))
            onChange(updated)
        },
        [variables, onChange],
    )

    const removeVariable = useCallback(
        (index: number) => {
            onChange(variables.filter((_, i) => i !== index))
        },
        [variables, onChange],
    )

    if (variables.length === 0) {
        return (
            <div className="border rounded-lg">
                <div className="flex flex-col items-center justify-center py-12">
                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted mb-3">
                        <Braces className="h-5 w-5 text-muted-foreground" />
                    </div>
                    <p className="font-medium text-sm mb-1">
                        {t("campaign.variables.empty_title", "No variables yet")}
                    </p>
                    <p className="text-xs text-muted-foreground text-center max-w-xs mb-4">
                        {t(
                            "campaign.variables.empty_description",
                            "Define variables like order_id or currency to use in your templates.",
                        )}
                    </p>
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={addVariable}
                        className="shadow-none"
                        type="button"
                    >
                        <Plus className="h-4 w-4 mr-2" />
                        {t("campaign.variables.add", "Add variable")}
                    </Button>
                </div>
            </div>
        )
    }

    return (
        <div className="border rounded-lg overflow-hidden">
            <div className="py-2 px-3">
                {variables.map((variable, index) => {
                    const nameError = validateName(variable.name, variables, index)

                    return (
                        <div key={index} className="py-1.5">
                            <div className="flex items-center">
                                <div className="inline-flex items-center flex-1 min-w-0">
                                    {/* Variable name */}
                                    <Input
                                        value={variable.name}
                                        onChange={(e) =>
                                            updateVariable(index, {
                                                name: e.target.value
                                                    .toLowerCase()
                                                    .replace(/\s/g, "_"),
                                            })
                                        }
                                        placeholder="name"
                                        className="h-8 w-36 rounded-r-none font-mono text-sm focus:z-10 shadow-none"
                                    />

                                    {/* Equals separator */}
                                    <div className="h-8 px-2 flex items-center border border-l-0 bg-muted/50 text-muted-foreground text-sm -ml-px">
                                        =
                                    </div>

                                    {/* Default value */}
                                    <Input
                                        value={variable.default ?? ""}
                                        onChange={(e) =>
                                            updateVariable(index, {
                                                default: e.target.value || undefined,
                                            })
                                        }
                                        placeholder="default (optional)"
                                        className="h-8 rounded-none border-l-0 text-sm flex-1 min-w-[120px] focus:z-10 -ml-px shadow-none"
                                    />

                                    {/* Delete button */}
                                    <button
                                        type="button"
                                        onClick={() => removeVariable(index)}
                                        className="h-8 w-8 flex items-center justify-center border border-l-0 rounded-r-md bg-background text-muted-foreground/60 hover:text-destructive hover:bg-destructive/5 transition-colors -ml-px shrink-0 cursor-pointer"
                                        aria-label={t(
                                            "campaign.variables.delete",
                                            "Delete variable",
                                        )}
                                    >
                                        <Trash2 className="h-3.5 w-3.5" />
                                    </button>
                                </div>
                            </div>
                            {nameError && variable.name && (
                                <p className="text-xs text-destructive mt-1 ml-1">{nameError}</p>
                            )}
                        </div>
                    )
                })}

                {/* Add button */}
                <div className="flex items-center py-1.5">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={addVariable}
                        className="h-8 text-xs shadow-none"
                        type="button"
                    >
                        <Plus className="h-3.5 w-3.5 mr-1.5" />
                        {t("campaign.variables.add", "Add variable")}
                    </Button>
                </div>
            </div>
        </div>
    )
}
