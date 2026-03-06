import { useEffect, useRef } from "react"
import type { UseFormReturn } from "react-hook-form"
import { snakeToTitle } from "@/utils"

import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { CodeEditor } from "@/components/ui/code-editor"
import { KeyValueEditor } from "@/components/ui/key-value-editor"
import { VariableAutocompleteInput } from "@/components/ui/variable-autocomplete-input"

export interface SchemaProperty {
    name: string
    schema: Schema
    hidden?: boolean
}

export interface Schema {
    type: "string" | "number" | "boolean" | "object"
    enum?: string[]
    title?: string
    description?: string
    properties?: SchemaProperty[] | Record<string, Schema>
    required?: string[]
    minLength?: number
    format?: string
    order?: number
}

/**
 * Normalize properties from either the new array format
 * `[{ name, schema }]` or the legacy object format `{ name: schema }`
 * into an ordered array of `[name, schema]` tuples.
 */
function normalizeProperties(
    properties: SchemaProperty[] | Record<string, Schema> | undefined,
): [string, Schema][] {
    if (!properties) return []
    if (Array.isArray(properties)) {
        return properties
            .filter((p) => !p.hidden)
            .map((p, i) => [p.name, { ...p.schema, order: p.schema.order ?? i }])
    }
    return Object.entries(properties)
}

export interface SchemaFieldsProps {
    title?: string
    description?: string
    parent: string
    schema: Schema
    form: UseFormReturn<any>
    variableNames?: string[]
}

export function SchemaFields({
    title,
    description,
    parent,
    form,
    schema,
    variableNames,
}: SchemaFieldsProps) {
    // Stable reference to form methods to avoid re-triggering the effect
    // when the form object identity changes across renders.
    const formRef = useRef(form)
    formRef.current = form

    const entries = normalizeProperties(schema?.properties)

    // Set default values for enum fields that haven't been touched yet
    useEffect(() => {
        if (!entries.length) return
        for (const [key, item] of entries) {
            if (item.enum && item.enum.length > 0) {
                const fieldName = `${parent}.${key}`
                const current = formRef.current.getValues(fieldName)
                if (!current) {
                    formRef.current.setValue(fieldName, item.enum[0], { shouldDirty: false })
                }
            }
        }
    }, [schema, parent])

    if (!entries.length) {
        return null
    }

    const sorted = [...entries].sort((a, b) => {
        const orderA = a[1].order ?? 0
        const orderB = b[1].order ?? 0
        return orderA - orderB
    })

    return (
        <div className="grid gap-4">
            {title && <h4 className="text-sm font-medium">{snakeToTitle(title)}</h4>}
            {description && <p className="text-sm text-muted-foreground">{description}</p>}
            {sorted.map(([key, item]) => {
                const required = schema.required?.includes(key)
                const fieldTitle = item.title ?? snakeToTitle(key)
                const fieldName = `${parent}.${key}`

                // format: "code" — render CodeEditor
                if (item.format === "code") {
                    return (
                        <div key={key} className="grid gap-2">
                            <Label className="inline-flex items-center gap-1">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            <CodeEditor
                                value={form.watch(fieldName) ?? ""}
                                onChange={(val) =>
                                    form.setValue(fieldName, val, { shouldDirty: true })
                                }
                                minHeight={120}
                                maxHeight={300}
                                variableNames={variableNames}
                            />
                        </div>
                    )
                }

                // format: "key-value" — render KeyValueEditor
                if (item.format === "key-value") {
                    return (
                        <div key={key} className="grid gap-2">
                            <Label className="inline-flex items-center gap-1">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            <KeyValueEditor
                                value={(form.watch(fieldName) as Record<string, string>) ?? {}}
                                onChange={(val) =>
                                    form.setValue(fieldName, val, { shouldDirty: true })
                                }
                                variableNames={variableNames}
                            />
                        </div>
                    )
                }

                if (item.enum) {
                    return (
                        <div key={key} className="grid gap-2">
                            <Label className="inline-flex items-center gap-1">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            <Select
                                value={form.watch(fieldName) ?? ""}
                                onValueChange={(val) =>
                                    form.setValue(fieldName, val, { shouldDirty: true })
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {item.enum.map((value: string) => (
                                        <SelectItem key={value} value={value}>
                                            {snakeToTitle(value)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    )
                } else if (item.type === "string" || item.type === "number") {
                    const useTextarea = (item.minLength ?? 0) >= 80
                    const useAutocomplete =
                        item.type === "string" &&
                        !useTextarea &&
                        variableNames &&
                        variableNames.length > 0
                    return (
                        <div key={key} className="grid gap-2">
                            <Label className="inline-flex items-center gap-1">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            {useTextarea ? (
                                <Textarea
                                    {...form.register(fieldName, {
                                        required,
                                        minLength: item.minLength,
                                    })}
                                />
                            ) : useAutocomplete ? (
                                <VariableAutocompleteInput
                                    variableNames={variableNames}
                                    value={form.watch(fieldName) ?? ""}
                                    onChange={(val) =>
                                        form.setValue(fieldName, val, { shouldDirty: true })
                                    }
                                />
                            ) : (
                                <Input
                                    type={item.type === "number" ? "number" : "text"}
                                    {...form.register(fieldName, {
                                        required,
                                        minLength: item.minLength,
                                        valueAsNumber: item.type === "number",
                                    })}
                                />
                            )}
                        </div>
                    )
                } else if (item.type === "boolean") {
                    return (
                        <div
                            key={key}
                            className="flex items-center justify-between rounded-lg border p-3"
                        >
                            <div className="space-y-0.5">
                                <Label>{fieldTitle}</Label>
                                {item.description && (
                                    <p className="text-sm text-muted-foreground">
                                        {item.description}
                                    </p>
                                )}
                            </div>
                            <Switch
                                checked={form.watch(fieldName) ?? false}
                                onCheckedChange={(checked) =>
                                    form.setValue(fieldName, checked, { shouldDirty: true })
                                }
                            />
                        </div>
                    )
                } else if (item.type === "object") {
                    return (
                        <SchemaFields
                            key={key}
                            form={form}
                            title={fieldTitle}
                            description={item.description}
                            parent={fieldName}
                            schema={item}
                            variableNames={variableNames}
                        />
                    )
                }
                return null
            })}
        </div>
    )
}
