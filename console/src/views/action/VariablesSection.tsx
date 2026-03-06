import { Controller, useFieldArray, type UseFormReturn } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { ChevronDown, Plus, Trash2 } from "lucide-react"

import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"

import { VARIABLE_TYPES, type ActionFormValues } from "./action-form-types"

interface VariablesSectionProps {
    form: UseFormReturn<ActionFormValues>
}

export function VariablesSection({ form }: VariablesSectionProps) {
    const { t } = useTranslation()
    const {
        fields: variableFields,
        append: appendVariable,
        remove: removeVariable,
    } = useFieldArray({
        control: form.control,
        name: "variables",
    })

    return (
        <Collapsible defaultOpen={variableFields.length > 0}>
            <CollapsibleTrigger asChild>
                <button
                    type="button"
                    className="flex w-full items-center justify-between rounded-lg border px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors"
                >
                    <span className="flex items-center gap-2">
                        {t("variables", "Variables")}
                        {variableFields.length > 0 && (
                            <Badge variant="secondary" className="text-xs px-1.5 py-0">
                                {variableFields.length}
                            </Badge>
                        )}
                    </span>
                    <ChevronDown className="h-4 w-4 text-muted-foreground transition-transform duration-200 [[data-state=open]>&]:rotate-180" />
                </button>
            </CollapsibleTrigger>
            <CollapsibleContent>
                <div className="border border-t-0 rounded-b-lg px-4 py-4 space-y-3">
                    {variableFields.map((field, index) => (
                        <div key={field.id} className="flex items-start gap-2">
                            <Controller
                                name={`variables.${index}.name`}
                                control={form.control}
                                render={({ field: nameField, fieldState }) => (
                                    <div className="w-2/5">
                                        <Input
                                            {...nameField}
                                            placeholder={t("variable_name", "name")}
                                            aria-invalid={fieldState.invalid}
                                            className="font-mono text-xs"
                                            autoComplete="off"
                                        />
                                    </div>
                                )}
                            />
                            <Controller
                                name={`variables.${index}.type`}
                                control={form.control}
                                render={({ field: typeField }) => (
                                    <Select
                                        value={typeField.value}
                                        onValueChange={typeField.onChange}
                                    >
                                        <SelectTrigger className="w-24 text-xs">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {VARIABLE_TYPES.map((t) => (
                                                <SelectItem key={t} value={t} className="text-xs">
                                                    {t}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                )}
                            />
                            <Controller
                                name={`variables.${index}.default`}
                                control={form.control}
                                render={({ field: defaultField }) => (
                                    <div className="flex-1">
                                        <Input
                                            {...defaultField}
                                            placeholder={t("default_value", "default")}
                                            className="text-xs"
                                            autoComplete="off"
                                        />
                                    </div>
                                )}
                            />
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-9 w-9 shrink-0 text-muted-foreground hover:text-destructive"
                                onClick={() => removeVariable(index)}
                            >
                                <Trash2 className="h-4 w-4" />
                            </Button>
                        </div>
                    ))}
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="w-full"
                        onClick={() => appendVariable({ name: "", type: "string", default: "" })}
                    >
                        <Plus className="h-4 w-4 mr-1.5" />
                        {t("add_variable", "Add Variable")}
                    </Button>
                </div>
            </CollapsibleContent>
        </Collapsible>
    )
}
