import { useCallback, useState } from "react"
import { useTranslation } from "react-i18next"
import { Braces, ChevronDown, Plus, Trash2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import type { CampaignVariable, ChannelType } from "@/types"

interface CampaignVariablesProps {
    variables: CampaignVariable[]
    onChange: (variables: CampaignVariable[]) => void
    channel?: ChannelType
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

function VariableExample({ children }: { children: string }) {
    return (
        <span className="inline-flex rounded border bg-background px-1.5 py-0.5 font-mono text-[11px] text-foreground">
            {children}
        </span>
    )
}

function BuiltInVariablesHelp({ channel }: { channel?: ChannelType }) {
    const { t } = useTranslation()
    const examples = [
        {
            title: t("campaign.variables.builtin.user.title", "Recipient"),
            description: t(
                "campaign.variables.builtin.user.description",
                "Profile fields and project schema fields.",
            ),
            values: ["user.email", "user.phone", "user.data.*"],
        },
        {
            title: t("campaign.variables.builtin.system.title", "System"),
            description: t(
                "campaign.variables.builtin.system.description",
                "Generated when the template is rendered.",
            ),
            values: ["now"],
        },
    ]

    if (channel === "email") {
        examples.splice(1, 0, {
            title: t("campaign.variables.builtin.links.title", "Email"),
            description: t(
                "campaign.variables.builtin.links.description",
                "Subscription links for email campaigns.",
            ),
            values: ["unsubscribe_url", "preferences_url"],
        })
    }

    return (
        <div className="border-t bg-muted/50 px-3 py-3 shadow-inner">
            <p className="mb-2 text-xs text-muted-foreground">
                {t(
                    "campaign.variables.builtin.description",
                    "Recipient, system, and email variables are already available in templates without adding them here.",
                )}
            </p>
            <div className="grid gap-2 sm:grid-cols-2">
                {examples.map((example, index) => (
                    <div
                        key={example.title}
                        className={`rounded-md border bg-background p-2 ${examples.length % 2 === 1 && index === examples.length - 1 ? "sm:col-span-2" : ""}`}
                    >
                        <div className="mb-1 flex items-center gap-2">
                            <p className="text-xs font-medium">{example.title}</p>
                        </div>
                        <p className="mb-2 text-xs text-muted-foreground">{example.description}</p>
                        <div className="flex flex-wrap gap-1.5">
                            {example.values.map((value) => (
                                <VariableExample key={value}>{value}</VariableExample>
                            ))}
                        </div>
                    </div>
                ))}
            </div>
            <p className="mt-3 flex flex-wrap items-center justify-center gap-1 text-center text-xs text-muted-foreground">
                {t(
                    "campaign.variables.custom.description",
                    "Custom variables you add here are used as",
                )}
                <VariableExample>{"campaign.variable_name"}</VariableExample>
            </p>
        </div>
    )
}

export function CampaignVariables({ variables, onChange, channel }: CampaignVariablesProps) {
    const { t } = useTranslation()
    const [showBuiltIns, setShowBuiltIns] = useState(false)

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

    const editor =
        variables.length === 0 ? (
            <div>
                <div className="flex flex-col items-center justify-center py-12">
                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted mb-3">
                        <Braces className="h-5 w-5 text-muted-foreground" />
                    </div>
                    <p className="font-medium text-sm mb-1">
                        {t("campaign.variables.empty_title", "No custom variables yet")}
                    </p>
                    <p className="text-xs text-muted-foreground text-center max-w-xs mb-4">
                        {t(
                            "campaign.variables.empty_description",
                            "Add one only when a template needs data passed from a journey or the API.",
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
                        {t("campaign.variables.add", "Add custom variable")}
                    </Button>
                </div>
            </div>
        ) : (
            <div>
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
                                    <p className="text-xs text-destructive mt-1 ml-1">
                                        {nameError}
                                    </p>
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
                            {t("campaign.variables.add", "Add custom variable")}
                        </Button>
                    </div>
                </div>
            </div>
        )

    return (
        <div className="border rounded-lg overflow-hidden">
            <div className="flex items-center justify-between gap-3 border-b bg-muted/30 px-3 py-2">
                <span className="text-sm font-medium text-muted-foreground">
                    {t("campaign.variables.custom.title", "Custom variables")}
                </span>
                <button
                    type="button"
                    onClick={() => setShowBuiltIns((value) => !value)}
                    className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
                    aria-expanded={showBuiltIns}
                >
                    {t("campaign.variables.builtin.toggle", "View defaults")}
                    <ChevronDown
                        className={`h-3.5 w-3.5 transition-transform ${showBuiltIns ? "rotate-180" : ""}`}
                    />
                </button>
            </div>
            {showBuiltIns && <BuiltInVariablesHelp channel={channel} />}
            {editor}
        </div>
    )
}
