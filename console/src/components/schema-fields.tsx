import { useEffect, useMemo, useRef } from "react"
import type { UseFormReturn } from "react-hook-form"
import { snakeToTitle } from "@/utils"

import { Input } from "@/components/ui/input"
import { TemplateInput } from "@/components/ui/template-input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Button } from "@/components/ui/button"
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

interface StringFieldWithFileProps {
    fieldKey: string
    item: Schema
    value: Record<string, unknown>
    set: (key: string, v: unknown) => void
    required: boolean
    fieldTitle: string
    variables?: VariableGroup[]
}

function StringFieldWithFile({
    fieldKey,
    item,
    value,
    set,
    required,
    fieldTitle,
    variables,
}: StringFieldWithFileProps) {
    const fileInputRef = useRef<HTMLInputElement>(null)
    const currentValue = (value[fieldKey] as string) ?? ""

    const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return

        const reader = new FileReader()
        reader.onload = () => {
            let result = reader.result as string
            if (item.requireBase64) {
                result = btoa(result)
            }
            set(fieldKey, result)
            if (fileInputRef.current) {
                fileInputRef.current.value = ""
            }
        }
        reader.readAsText(file)
    }

    const handleBase64Encode = () => {
        if (currentValue && !isBase64(currentValue)) {
            set(fieldKey, btoa(currentValue))
        }
    }

    const isBase64 = (str: string): boolean => {
        if (!str) return false
        try {
            return btoa(atob(str)) === str
        } catch {
            return false
        }
    }

    const useTextarea = (item.minLength ?? 0) >= 80
    const useTemplateInput =
        !useTextarea &&
        !item.fileUpload &&
        variables &&
        variables.some((g) => g.variables.length > 0)

    return (
        <div className="grid gap-1.5">
            <Label className="inline-flex items-center gap-1 text-sm font-medium">
                {fieldTitle}
                {required && <span className="text-destructive">*</span>}
            </Label>
            {item.description && (
                <p className="text-sm text-muted-foreground">{item.description}</p>
            )}
            <div className="flex gap-2">
                <div className="flex-1">
                    {useTextarea ? (
                        <Textarea
                            value={currentValue}
                            onChange={(e) => set(fieldKey, e.target.value)}
                            placeholder={item.preview}
                        />
                    ) : useTemplateInput ? (
                        <TemplateInput
                            value={currentValue}
                            onChange={(val) => set(fieldKey, val)}
                            placeholder={item.preview}
                            variables={variables}
                        />
                    ) : (
                        <Input
                            type={item.format === "password" ? "password" : "text"}
                            value={currentValue}
                            onChange={(e) => set(fieldKey, e.target.value)}
                            placeholder={item.preview}
                        />
                    )}
                </div>
                {item.fileUpload && (
                    <>
                        <input
                            type="file"
                            ref={fileInputRef}
                            accept={item.fileAccept}
                            className="hidden"
                            onChange={handleFileUpload}
                        />
                        <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => fileInputRef.current?.click()}
                        >
                            Upload File
                        </Button>
                    </>
                )}
                {item.requireBase64 && !item.fileUpload && currentValue && !isBase64(currentValue) && (
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={handleBase64Encode}
                    >
                        Base64 Encode
                    </Button>
                )}
            </div>
        </div>
    )
}

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
    requireBase64?: boolean
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
}

export function SchemaFields({
    title,
    description,
    schema,
    value,
    onChange,
    variables,
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
                        </div>
                    )
                }

                // string / number
                if (item.type === "string" || item.type === "number") {
                    // String fields with file upload or base64 encoding
                    if (item.type === "string" && (item.fileUpload || item.requireBase64)) {
                        return (
                            <StringFieldWithFile
                                key={key}
                                fieldKey={key}
                                item={item}
                                value={value}
                                set={set}
                                required={required ?? false}
                                fieldTitle={fieldTitle}
                                variables={variables}
                            />
                        )
                    }

                    // Regular string/number fields
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
                            />
                        </div>
                    )
                }

                return null
            })}
        </div>
    )
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
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    form: UseFormReturn<any>
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

    // Derive valid keys from the current schema so we can clean up stale ones
    const schemaKeys = useMemo(() => {
        const entries = normalizeProperties(schema?.properties)
        return new Set(entries.map(([k]) => k))
    }, [schema])

    return (
        <SchemaFields
            title={title}
            description={description}
            schema={schema}
            value={watched}
            onChange={(next) => {
                // Diff and set only the keys that changed so we don't
                // unnecessarily mark untouched fields as dirty.
                for (const key of Object.keys(next)) {
                    const fieldName = `${parent}.${key}`
                    if (next[key] !== watched[key]) {
                        form.setValue(fieldName, next[key], { shouldDirty: true })
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
