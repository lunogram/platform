import { useEffect, useMemo } from "react"
import { snakeToTitle } from "@/utils"

import { Input } from "@/components/ui/input"
import { TemplateInput } from "@/components/ui/template-input"
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
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"

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
    preview?: string
    fileUpload?: boolean
    fileAccept?: string
}

function fileToBase64(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => {
            if (typeof reader.result !== "string") {
                reject(new Error("failed to read file as base64"))
                return
            }
            const comma = reader.result.indexOf(",")
            resolve(comma >= 0 ? reader.result.slice(comma + 1) : reader.result)
        }
        reader.onerror = () => reject(reader.error ?? new Error("failed to read file"))
        reader.readAsDataURL(file)
    })
}

/**
 * Normalize properties from either the array format `[{ name, schema }]`
 * or the legacy object format `{ name: schema }` into ordered tuples.
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
    /** Optional heading rendered above the fields. */
    title?: string
    /** Optional description rendered below the title. */
    description?: string
    /** The JSON schema describing the fields. */
    schema: Schema
    /** Current values keyed by property name. */
    value: Record<string, unknown>
    /** Called when any field value changes. */
    onChange: (v: Record<string, unknown>) => void
    /** Optional variable groups for TemplateInput / CodeEditor pickers. */
    variables?: VariableGroup[]
    /** Optional field-level error messages keyed by property name. */
    errors?: Record<string, string | undefined>
}

export function SchemaFields({
    title,
    description,
    schema,
    value,
    onChange,
    variables,
    errors,
}: SchemaFieldsProps) {
    const entries = normalizeProperties(schema?.properties)

    const entryKeys = useMemo(() => entries.map(([k]) => k).join(","), [entries])

    // Auto-select the first enum value for any unset enum fields.
    useEffect(() => {
        if (!entries.length) return
        const defaults: Record<string, unknown> = {}
        for (const [key, item] of entries) {
            if (item.enum && item.enum.length > 0 && !value[key]) {
                defaults[key] = item.enum[0]
            }
        }
        if (Object.keys(defaults).length > 0) {
            onChange({ ...value, ...defaults })
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [entryKeys])

    // Initialize missing boolean fields to false.
    useEffect(() => {
        if (!entries.length) return
        let changed = false
        const next = { ...value }
        for (const [key, item] of entries) {
            if (item.type === "boolean" && next[key] === undefined) {
                next[key] = false
                changed = true
            }
        }
        if (changed) onChange(next)
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [entryKeys])

    if (!entries.length) return null

    const sorted = [...entries].sort((a, b) => (a[1].order ?? 0) - (b[1].order ?? 0))

    const set = (key: string, v: unknown) => onChange({ ...value, [key]: v })

    return (
        <div className="grid gap-3">
            {title && <h4 className="text-sm font-medium">{snakeToTitle(title)}</h4>}
            {description && <p className="text-sm text-muted-foreground">{description}</p>}
            {sorted.map(([key, item]) => {
                const required = schema.required?.includes(key)
                const fieldTitle = item.title ?? snakeToTitle(key)

                if (item.type === "string" && item.fileUpload) {
                    return (
                        <div key={key} className="grid gap-1.5">
                            <Label className="inline-flex items-center gap-1 text-sm font-medium">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            <Input
                                type="file"
                                accept={item.fileAccept}
                                onChange={async (e) => {
                                    const file = e.target.files?.[0]
                                    if (!file) return
                                    try {
                                        const base64 = await fileToBase64(file)
                                        set(key, base64)
                                    } catch (error) {
                                        console.error(error)
                                    }
                                }}
                            />
                            {Boolean(value[key]) && (
                                <p className="text-xs text-muted-foreground">File configured</p>
                            )}
                            {errors?.[key] && (
                                <p className="text-xs text-destructive">{errors[key]}</p>
                            )}
                        </div>
                    )
                }

                // format: "code"
                if (item.format === "code") {
                    return (
                        <div key={key} className="grid gap-1.5">
                            <Label className="inline-flex items-center gap-1 text-sm font-medium">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            <CodeEditor
                                value={(value[key] as string) ?? ""}
                                onChange={(val) => set(key, val)}
                                minHeight={120}
                                maxHeight={300}
                                variables={variables}
                            />
                            {errors?.[key] && (
                                <p className="text-xs text-destructive">{errors[key]}</p>
                            )}
                        </div>
                    )
                }

                // format: "key-value"
                if (item.format === "key-value") {
                    return (
                        <div key={key} className="grid gap-1.5">
                            <Label className="inline-flex items-center gap-1 text-sm font-medium">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            <KeyValueEditor
                                value={(value[key] as Record<string, string>) ?? {}}
                                onChange={(val) => set(key, val)}
                                variables={variables}
                            />
                            {errors?.[key] && (
                                <p className="text-xs text-destructive">{errors[key]}</p>
                            )}
                        </div>
                    )
                }

                // enum (select)
                if (item.enum) {
                    return (
                        <div key={key} className="grid gap-1.5">
                            <Label className="inline-flex items-center gap-1 text-sm font-medium">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            <Select
                                value={(value[key] as string) ?? ""}
                                onValueChange={(val) => set(key, val)}
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {item.enum.map((v: string) => (
                                        <SelectItem key={v} value={v}>
                                            {snakeToTitle(v)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                            {errors?.[key] && (
                                <p className="text-xs text-destructive">{errors[key]}</p>
                            )}
                        </div>
                    )
                }

                // string / number
                if (item.type === "string" || item.type === "number") {
                    const useTextarea = (item.minLength ?? 0) >= 80
                    const useTemplateInput =
                        item.type === "string" &&
                        !useTextarea &&
                        variables &&
                        variables.some((g) => g.variables.length > 0)
                    return (
                        <div key={key} className="grid gap-1.5">
                            <Label className="inline-flex items-center gap-1 text-sm font-medium">
                                {fieldTitle}
                                {required && <span className="text-destructive">*</span>}
                            </Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            {useTextarea ? (
                                <Textarea
                                    value={(value[key] as string) ?? ""}
                                    onChange={(e) => set(key, e.target.value)}
                                    placeholder={item.preview}
                                />
                            ) : useTemplateInput ? (
                                <TemplateInput
                                    value={(value[key] as string) ?? ""}
                                    onChange={(val) => set(key, val)}
                                    placeholder={item.preview}
                                    variables={variables}
                                />
                            ) : (
                                <Input
                                    type={item.type === "number" ? "number" : "text"}
                                    value={(value[key] as string | number) ?? ""}
                                    onChange={(e) =>
                                        set(
                                            key,
                                            item.type === "number"
                                                ? e.target.value === ""
                                                    ? undefined
                                                    : Number(e.target.value)
                                                : e.target.value,
                                        )
                                    }
                                    placeholder={item.preview}
                                />
                            )}
                            {errors?.[key] && (
                                <p className="text-xs text-destructive">{errors[key]}</p>
                            )}
                        </div>
                    )
                }

                // boolean
                if (item.type === "boolean") {
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
                                checked={(value[key] as boolean) ?? false}
                                onCheckedChange={(checked) => set(key, checked)}
                            />
                            {errors?.[key] && (
                                <p className="text-xs text-destructive">{errors[key]}</p>
                            )}
                        </div>
                    )
                }

                // nested object
                if (item.type === "object" && item.properties) {
                    return (
                        <div key={key} className="grid gap-1.5">
                            <Label className="text-sm font-medium">{fieldTitle}</Label>
                            {item.description && (
                                <p className="text-sm text-muted-foreground">{item.description}</p>
                            )}
                            <SchemaFields
                                schema={item}
                                value={(value[key] as Record<string, unknown>) ?? {}}
                                onChange={(nested) => set(key, nested)}
                                variables={variables}
                                errors={errors}
                            />
                            {errors?.[key] && (
                                <p className="text-xs text-destructive">{errors[key]}</p>
                            )}
                        </div>
                    )
                }

                return null
            })}
        </div>
    )
}

interface FormAdapter {
    watch(name: string): Record<string, unknown> | undefined
    setValue(name: string, value: unknown, options?: { shouldDirty?: boolean }): void
    clearErrors(name?: string | string[] | readonly string[]): void
    formState: { errors: Record<string, unknown> }
}

export interface FormSchemaFieldsProps {
    /** Optional heading rendered above the fields. */
    title?: string
    /** Optional description rendered below the title. */
    description?: string
    /** The form field path prefix (e.g. "config"). */
    parent: string
    /** The JSON schema describing the fields. */
    schema: Schema
    /** The react-hook-form instance. */
    form: FormAdapter
}

/**
 * Adapter that bridges `SchemaFields` to a `react-hook-form` instance.
 *
 * It reads/writes values through `form.watch` / `form.setValue` so the
 * underlying form state stays in sync.
 */
export function FormSchemaFields({
    title,
    description,
    parent,
    form,
    schema,
}: FormSchemaFieldsProps) {
    const watched = form.watch(parent) ?? {}
    const formErrors = form.formState.errors

    // Derive valid keys from the current schema so we can clean up stale ones
    const schemaKeys = useMemo(() => {
        const entries = normalizeProperties(schema?.properties)
        return new Set(entries.map(([k]) => k))
    }, [schema])

    // Extract field-level errors from react-hook-form for the parent path
    const errors = useMemo(() => {
        const parentErrors = formErrors[parent]
        if (!parentErrors || typeof parentErrors !== "object") return undefined
        const result: Record<string, string | undefined> = {}
        for (const [key, err] of Object.entries(
            parentErrors as Record<string, { message?: string }>,
        )) {
            if (err?.message) {
                result[key] = err.message
            }
        }
        return Object.keys(result).length > 0 ? result : undefined
    }, [formErrors, parent])

    return (
        <SchemaFields
            title={title}
            description={description}
            schema={schema}
            value={watched}
            errors={errors}
            onChange={(next) => {
                // Diff and set only the keys that changed so we don't
                // unnecessarily mark untouched fields as dirty.
                for (const key of Object.keys(next)) {
                    const fieldName = `${parent}.${key}`
                    if (next[key] !== watched[key]) {
                        form.setValue(fieldName, next[key], { shouldDirty: true })
                        form.clearErrors(fieldName)
                    }
                }
                // Remove stale keys that no longer exist in the current schema
                for (const key of Object.keys(watched)) {
                    if (!schemaKeys.has(key) && !(key in next)) {
                        form.setValue(`${parent}.${key}`, undefined, { shouldDirty: true })
                    }
                }
            }}
        />
    )
}
