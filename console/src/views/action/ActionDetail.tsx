import { Controller, useForm, useFieldArray } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams } from "react-router"
import { useContext, useEffect, useState } from "react"
import { Plus, Trash2, ArrowLeft } from "lucide-react"
import * as z from "zod"

import api from "@/api"
import { ProjectContext } from "@/contexts"
import type { Action } from "@/types"
import type { UUID } from "@/types/common"

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"

const methods = ["DELETE", "GET", "PATCH", "POST", "PUT"] as const

const actionFormSchema = z.object({
    name: z.string().min(1, "Name is required"),
    type: z.enum(["webhook"] as const),
    method: z.enum(methods),
    endpoint: z.string().min(1, "Endpoint is required"),
    body: z.string().optional(),
    headers: z.array(z.object({
        key: z.string(),
        value: z.string(),
    })).optional(),
})

type ActionFormValues = z.infer<typeof actionFormSchema>

function headersToArray(headers?: Record<string, string>): { key: string; value: string }[] {
    if (!headers || Object.keys(headers).length === 0) {
        return [{ key: "", value: "" }]
    }
    return Object.entries(headers).map(([key, value]) => ({ key, value }))
}

function bodyToString(body?: Record<string, unknown> | string): string {
    if (!body) return ""
    if (typeof body === "string") return body
    if (Object.keys(body).length === 0) return ""
    try {
        return JSON.stringify(body, null, 2)
    } catch {
        return ""
    }
}

const methodColors: Record<string, string> = {
    GET: "bg-emerald-600 text-white",
    POST: "bg-blue-600 text-white",
    PUT: "bg-amber-600 text-white",
    PATCH: "bg-orange-600 text-white",
    DELETE: "bg-red-600 text-white",
}

function buildCurlCommand(method: string, endpoint: string, headers: { key: string; value: string }[], body?: string): string {
    const parts: string[] = ["curl"]

    if (method !== "GET") {
        parts.push(`-X ${method}`)
    }

    parts.push(`'${endpoint || "https://..."}'`)

    const validHeaders = headers.filter(h => h.key.trim())
    for (const header of validHeaders) {
        parts.push(`-H '${header.key}: ${header.value}'`)
    }

    if (body && body.trim() && method !== "GET" && method !== "DELETE") {
        parts.push(`-d '${body}'`)
    }

    return parts.join(" \\\n  ")
}

export default function ActionDetail() {
    const [project] = useContext(ProjectContext)
    const { t } = useTranslation()
    const navigate = useNavigate()
    const { entityId } = useParams()
    const isNew = !entityId || entityId === 'new'
    const [isSaving, setIsSaving] = useState(false)

    const [existingAction, setExistingAction] = useState<Action | null>(null)

    useEffect(() => {
        if (!isNew && entityId) {
            api.actions.get(project.id, entityId as UUID)
                .then(setExistingAction)
                .catch(() => navigate(`/projects/${project.id}/actions`))
        }
    }, [isNew, entityId, project.id, navigate])

    const form = useForm<ActionFormValues>({
        resolver: zodResolver(actionFormSchema),
        defaultValues: {
            name: "",
            type: "webhook",
            method: "POST",
            endpoint: "",
            body: "",
            headers: [{ key: "", value: "" }],
        },
    })

    useEffect(() => {
        if (existingAction) {
            form.reset({
                name: existingAction.name,
                type: existingAction.type,
                method: existingAction.config?.method ?? "POST",
                endpoint: existingAction.config?.endpoint ?? "",
                body: bodyToString(existingAction.config?.body),
                headers: headersToArray(existingAction.config?.headers),
            })
        }
    }, [existingAction, form])

    const { fields, append, remove } = useFieldArray({
        control: form.control,
        name: "headers",
    })

    const method = form.watch("method") ?? "POST"
    const endpoint = form.watch("endpoint") ?? ""
    const headers = form.watch("headers") ?? []
    const body = form.watch("body") ?? ""

    const curlCommand = buildCurlCommand(method, endpoint, headers, body)

    const onSubmit = async (data: ActionFormValues) => {
        setIsSaving(true)
        try {
            const validHeaders = (data.headers ?? []).filter(h => h.key.trim())
            const headersObj: Record<string, string> = {}
            for (const h of validHeaders) {
                headersObj[h.key] = h.value
            }

            let bodyObj: Record<string, unknown> | undefined
            if (data.body && data.body.trim()) {
                try {
                    bodyObj = JSON.parse(data.body)
                } catch {
                    bodyObj = undefined
                }
            }

            const config: Record<string, unknown> = {
                method: data.method,
                endpoint: data.endpoint,
            }
            if (Object.keys(headersObj).length > 0) {
                config.headers = headersObj
            }
            if (bodyObj) {
                config.body = bodyObj
            }

            if (isNew) {
                await api.actions.create(project.id, {
                    name: data.name,
                    type: data.type,
                    config,
                })
                navigate(`/projects/${project.id}/actions`)
            } else {
                await api.actions.update(project.id, entityId as UUID, {
                    name: data.name,
                    type: data.type,
                    config,
                })
            }
        } finally {
            setIsSaving(false)
        }
    }

    if (!isNew && !existingAction) {
        return null
    }

    return (
        <div className="flex flex-col gap-6 p-6 max-w-4xl">
            {/* Header */}
            <div className="flex items-center gap-3">
                <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => navigate(`/projects/${project.id}/actions`)}
                >
                    <ArrowLeft className="h-4 w-4" />
                </Button>
                <h1 className="text-2xl font-semibold tracking-tight">
                    {isNew ? t('create_action', 'Create Action') : t('edit_action', 'Edit Action')}
                </h1>
            </div>

            <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-8">
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
                    {/* Left: Form */}
                    <div className="space-y-6">
                        <FieldGroup>
                            <Controller
                                name="name"
                                control={form.control}
                                render={({ field, fieldState }) => (
                                    <Field data-invalid={fieldState.invalid} className="gap-2">
                                        <FieldLabel>{t('name')} <span className="inline text-destructive">*</span></FieldLabel>
                                        <Input
                                            {...field}
                                            aria-invalid={fieldState.invalid}
                                            placeholder={t('action_name_placeholder', 'My Webhook Action')}
                                            autoComplete="off"
                                        />
                                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                                    </Field>
                                )}
                            />

                            <Controller
                                name="type"
                                control={form.control}
                                render={({ field }) => (
                                    <Field className="gap-2">
                                        <FieldLabel>{t('type')}</FieldLabel>
                                        <Select
                                            value={field.value}
                                            onValueChange={field.onChange}
                                        >
                                            <SelectTrigger>
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                <SelectItem value="webhook">Webhook</SelectItem>
                                            </SelectContent>
                                        </Select>
                                    </Field>
                                )}
                            />
                        </FieldGroup>

                        <div className="border-t pt-6">
                            <h2 className="text-lg font-medium mb-4">{t('webhook_configuration', 'Webhook Configuration')}</h2>

                            <FieldGroup>
                                <Controller
                                    name="method"
                                    control={form.control}
                                    render={({ field, fieldState }) => (
                                        <Field data-invalid={fieldState.invalid} className="gap-2">
                                            <FieldLabel>{t('method')}</FieldLabel>
                                            <Select
                                                value={field.value}
                                                onValueChange={field.onChange}
                                            >
                                                <SelectTrigger>
                                                    <SelectValue />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    {methods.map((m) => (
                                                        <SelectItem key={m} value={m}>{m}</SelectItem>
                                                    ))}
                                                </SelectContent>
                                            </Select>
                                            {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                                        </Field>
                                    )}
                                />

                                <Controller
                                    name="endpoint"
                                    control={form.control}
                                    render={({ field, fieldState }) => (
                                        <Field data-invalid={fieldState.invalid} className="gap-2">
                                            <FieldLabel>{t('endpoint')} <span className="inline text-destructive">*</span></FieldLabel>
                                            <Input
                                                {...field}
                                                aria-invalid={fieldState.invalid}
                                                placeholder="https://api.example.com/webhook"
                                                autoComplete="off"
                                            />
                                            {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                                        </Field>
                                    )}
                                />

                                <Field className="gap-2">
                                    <div className="flex items-center justify-between">
                                        <FieldLabel>{t('headers', 'Headers')}</FieldLabel>
                                        <Button
                                            type="button"
                                            variant="ghost"
                                            size="sm"
                                            onClick={() => append({ key: "", value: "" })}
                                        >
                                            <Plus className="w-4 h-4 mr-1" />
                                            {t('add_header', 'Add Header')}
                                        </Button>
                                    </div>
                                    <div className="space-y-2">
                                        {fields.map((item, index) => (
                                            <div key={item.id} className="flex items-center gap-2">
                                                <Controller
                                                    name={`headers.${index}.key`}
                                                    control={form.control}
                                                    render={({ field }) => (
                                                        <Input
                                                            {...field}
                                                            placeholder="Header name"
                                                            autoComplete="off"
                                                            className="flex-1"
                                                        />
                                                    )}
                                                />
                                                <Controller
                                                    name={`headers.${index}.value`}
                                                    control={form.control}
                                                    render={({ field }) => (
                                                        <Input
                                                            {...field}
                                                            placeholder="Value"
                                                            autoComplete="off"
                                                            className="flex-1"
                                                        />
                                                    )}
                                                />
                                                {fields.length > 1 && (
                                                    <Button
                                                        type="button"
                                                        variant="ghost"
                                                        size="icon"
                                                        onClick={() => remove(index)}
                                                        className="shrink-0"
                                                    >
                                                        <Trash2 className="w-4 h-4 text-muted-foreground" />
                                                    </Button>
                                                )}
                                            </div>
                                        ))}
                                    </div>
                                </Field>

                                <Controller
                                    name="body"
                                    control={form.control}
                                    render={({ field, fieldState }) => (
                                        <Field data-invalid={fieldState.invalid} className="gap-2">
                                            <FieldLabel>{t('body')}</FieldLabel>
                                            <Textarea
                                                {...field}
                                                aria-invalid={fieldState.invalid}
                                                placeholder='{"key": "value"}'
                                                autoComplete="off"
                                                className="font-mono text-sm min-h-[120px]"
                                            />
                                            {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                                        </Field>
                                    )}
                                />

                            </FieldGroup>
                        </div>
                    </div>

                    {/* Right: Preview */}
                    <div className="space-y-4">
                        <h2 className="text-lg font-medium">{t('preview')}</h2>
                        <div className="bg-zinc-800/90 rounded-lg shadow-lg overflow-hidden border border-zinc-600/50">
                            <div className="flex items-center gap-2 px-4 py-2.5 bg-zinc-700/60 border-b border-zinc-600/40">
                                <div className="flex gap-1.5">
                                    <div className="w-2.5 h-2.5 rounded-full bg-red-400/80" />
                                    <div className="w-2.5 h-2.5 rounded-full bg-yellow-400/80" />
                                    <div className="w-2.5 h-2.5 rounded-full bg-green-400/80" />
                                </div>
                                <div className="flex-1 flex items-center gap-2 ml-2">
                                    <Badge className={`${methodColors[method] ?? "bg-zinc-600 text-white"} text-[10px] px-1.5 py-0 font-bold rounded border-0`}>
                                        {method}
                                    </Badge>
                                    <span className="text-zinc-400 text-xs font-mono truncate">
                                        {endpoint || "https://..."}
                                    </span>
                                </div>
                            </div>
                            <pre className="px-5 py-5 text-sm font-mono text-green-400/90 whitespace-pre-wrap break-all overflow-auto min-h-[200px] max-h-[500px] leading-relaxed">
                                <span className="text-zinc-500">$ </span>{curlCommand}
                            </pre>
                        </div>
                    </div>
                </div>

                {/* Actions */}
                <div className="flex items-center gap-3 border-t pt-6">
                    <Button type="submit" disabled={isSaving}>
                        {isSaving
                            ? t('saving', 'Saving...')
                            : isNew
                                ? t('create_action', 'Create Action')
                                : t('save_action', 'Save Action')
                        }
                    </Button>
                    <Button
                        type="button"
                        variant="outline"
                        onClick={() => navigate(`/projects/${project.id}/actions`)}
                    >
                        {t('cancel')}
                    </Button>
                </div>
            </form>
        </div>
    )
}
