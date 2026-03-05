import { Controller, useFieldArray, useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams } from "react-router"
import { useCallback, useContext, useEffect, useMemo, useRef, useState } from "react"
import { ArrowLeft, ChevronDown, Plus, Trash2 } from "lucide-react"
import * as z from "zod"

import oapiClient, {
    type Action,
    type ActionMeta,
    type TestActionResult,
} from "@/oapi/client"
import { ProjectContext } from "@/contexts"
import { useResolver } from "@/hooks"

import { SchemaFields, type Schema } from "@/components/SchemaFields"

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { CodeEditor } from "@/components/ui/code-editor"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { cn, snakeToTitle } from "@/utils"

function TestResultPanel({ result }: { result: TestActionResult }) {
    const isError = result.status === "error"
    const hasMetadata = result.metadata && Object.keys(result.metadata).length > 0
    const statusLabel = result.status || "unknown"

    const metadataJson = hasMetadata
        ? JSON.stringify(result.metadata, null, 2)
        : ""

    return (
        <div className="space-y-4">
            {/* Status + status code */}
            <div className="flex items-center gap-3">
                <Badge variant={isError ? "destructive" : "default"}>
                    {statusLabel}
                </Badge>
                {result.status_code != null && (
                    <span className="text-sm text-muted-foreground">
                        HTTP {result.status_code}
                    </span>
                )}
            </div>

            {/* Error message */}
            {result.error && (
                <div className="rounded-lg border border-destructive/50 bg-destructive/5 px-4 py-3 text-sm text-destructive">
                    {result.error}
                </div>
            )}

            {/* Metadata */}
            {metadataJson ? (
                <CodeEditor
                    value={metadataJson}
                    onChange={() => {}}
                    readOnly
                    minHeight={80}
                    maxHeight={400}
                />
            ) : (
                <p className="text-sm text-muted-foreground">No response data returned.</p>
            )}
        </div>
    )
}

const VARIABLE_TYPES = ["string", "bool", "int"] as const
type VariableType = (typeof VARIABLE_TYPES)[number]

const actionFormSchema = z.object({
    name: z.string().min(1, "Name is required"),
    config: z.record(z.string(), z.unknown()).optional(),
    payload: z.record(z.string(), z.unknown()).optional(),
    variables: z.array(z.object({
        name: z.string().min(1, "Variable name is required"),
        type: z.enum(VARIABLE_TYPES),
        default: z.string(),
    })).optional(),
})

type ActionFormValues = z.infer<typeof actionFormSchema>

type StoredVariable = { name: string; type: VariableType; default?: string }

/** Convert stored variables array to form field array */
function storedToForm(stored?: StoredVariable[]): { name: string; type: VariableType; default: string }[] {
    if (!stored || !Array.isArray(stored)) return []
    return stored.map((v) => ({ name: v.name, type: v.type ?? "string", default: v.default ?? "" }))
}

/** Convert form variables to a Record<string, any> using default values (for test/preview) */
function variablesToMap(arr?: { name: string; type: VariableType; default: string }[]): Record<string, unknown> {
    if (!arr) return {}
    const result: Record<string, unknown> = {}
    for (const v of arr) {
        if (!v.name.trim()) continue
        const key = v.name.trim()
        const raw = v.default
        switch (v.type) {
            case "int":
                result[key] = raw === "" ? 0 : Number(raw)
                break
            case "bool":
                result[key] = raw === "true" || raw === "1"
                break
            default:
                result[key] = raw
        }
    }
    return result
}

export default function ActionDetail() {
    const [project] = useContext(ProjectContext)
    const { t } = useTranslation()
    const navigate = useNavigate()
    const { entityId, type: routeType } = useParams()
    const isNew = !entityId || entityId === "new"

    // Type is determined at creation time (from route) or from the existing action
    const actionType = isNew ? routeType : undefined

    const [isSaving, setIsSaving] = useState(false)
    const [isTesting, setIsTesting] = useState(false)
    const [testResult, setTestResult] = useState<TestActionResult | null>(null)
    const [activeTab, setActiveTab] = useState<string>("preview")
    const [actionMetas] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                params: { path: { projectID: project.id } },
            })
            return data ?? null
        }, [project.id]),
    )
    const [existingAction, setExistingAction] = useState<Action | null>(null)

    useEffect(() => {
        if (!isNew && entityId) {
            oapiClient
                .GET("/api/admin/projects/{projectID}/actions/{actionID}", {
                    params: {
                        path: { projectID: project.id, actionID: entityId },
                    },
                })
                .then(({ data }) => {
                    if (data) setExistingAction(data)
                    else navigate(`/projects/${project.id}/actions`)
                })
                .catch(() => navigate(`/projects/${project.id}/actions`))
        }
    }, [isNew, entityId, project.id, navigate])

    // Resolve the effective type: route param for new, existing action for edit
    const selectedType = isNew ? actionType : existingAction?.type
    const selectedMeta = actionMetas?.find((m: ActionMeta) => m.type === selectedType) ?? null

    // Redirect if no valid type for new actions
    useEffect(() => {
        if (isNew && !actionType) {
            navigate(`/projects/${project.id}/actions`)
        }
    }, [isNew, actionType, project.id, navigate])

    const form = useForm<ActionFormValues>({
        resolver: zodResolver(actionFormSchema),
        defaultValues: {
            name: "",
            config: {},
            payload: {},
            variables: [],
        },
    })

    const { fields: variableFields, append: appendVariable, remove: removeVariable } = useFieldArray({
        control: form.control,
        name: "variables",
    })

    useEffect(() => {
        if (existingAction && actionMetas) {
            const stored = (existingAction.config ?? {}) as Record<string, unknown>
            form.reset({
                name: existingAction.name,
                config: (stored.config as Record<string, unknown>) ?? {},
                payload: (stored.payload as Record<string, unknown>) ?? {},
                variables: storedToForm(stored.variables as StoredVariable[]),
            })
        }
    }, [existingAction, actionMetas, form])

    const iframeRef = useRef<HTMLIFrameElement>(null)
    const [iframeHeight, setIframeHeight] = useState(300)
    const [previewHtml, setPreviewHtml] = useState<string | null>(null)
    const iframeLoadedRef = useRef(false)

    // Fetch preview HTML when action type changes
    useEffect(() => {
        if (!selectedType) {
            setPreviewHtml(null)
            return
        }
        iframeLoadedRef.current = false
        fetch(`/api/admin/projects/${project.id}/actions/meta/${selectedType}/preview`)
            .then((r) => {
                if (r.ok) return r.text()
                return null
            })
            .then((html) => setPreviewHtml(html ?? null))
            .catch(() => setPreviewHtml(null))
    }, [selectedType, project.id])

    // Post form data to iframe on changes — useWatch returns new references on nested changes
    const config = useWatch({ control: form.control, name: "config" })
    const payload = useWatch({ control: form.control, name: "payload" })
    const variables = useWatch({ control: form.control, name: "variables" })

    // Derive variable names for autocomplete in payload fields
    const variableNames = useMemo(
        () => (variables ?? []).map((v) => v.name.trim()).filter(Boolean),
        [variables],
    )

    // Serialize to JSON so the effect fires on deep changes
    const previewData = JSON.stringify({
        config: { ...(config ?? {}), ...(payload ?? {}) },
        payload: payload ?? {},
        variables: variablesToMap(variables),
    })

    const postToIframe = useCallback(() => {
        if (!iframeRef.current?.contentWindow || !iframeLoadedRef.current) return
        iframeRef.current.contentWindow.postMessage(
            {
                type: "preview-update",
                actionType: selectedType,
                ...JSON.parse(previewData),
            },
            "*",
        )
    }, [selectedType, previewData])

    // Keep a ref to the latest postToIframe so the "preview-ready" handler
    // always calls the most current version (avoids stale closure from
    // the race between form.reset and iframe mount).
    const postToIframeRef = useRef(postToIframe)
    useEffect(() => {
        postToIframeRef.current = postToIframe
    }, [postToIframe])

    // Post data when it changes (only if iframe is loaded)
    useEffect(() => {
        if (!previewHtml) return
        postToIframe()
    }, [previewHtml, postToIframe])

    // Re-post data when iframe finishes loading (DOM ready)
    const handleIframeLoad = useCallback(() => {
        iframeLoadedRef.current = true
    }, [])

    // Listen for messages from iframe: resize and preview-ready
    useEffect(() => {
        const handler = (e: MessageEvent) => {
            if (e.data?.type === "resize" && typeof e.data.height === "number") {
                setIframeHeight(e.data.height)
            }
            // The Preact app inside the iframe signals it has mounted its
            // message listener. Post the current form data in response so
            // the preview is guaranteed to receive it.
            if (e.data?.type === "preview-ready") {
                iframeLoadedRef.current = true
                postToIframeRef.current()
            }
        }
        window.addEventListener("message", handler)
        return () => window.removeEventListener("message", handler)
    }, [])

    const onSubmit = async (data: ActionFormValues) => {
        if (!selectedType) return
        setIsSaving(true)
        try {
            const vars = (data.variables ?? []).filter((v) => v.name.trim())
            const actionConfig: Record<string, unknown> = {
                config: data.config ?? {},
                payload: data.payload ?? {},
            }
            if (vars.length > 0) {
                actionConfig.variables = vars
            }

            const body = {
                name: data.name,
                type: selectedType,
                config: actionConfig,
            }

            if (isNew) {
                await oapiClient.POST("/api/admin/projects/{projectID}/actions", {
                    params: { path: { projectID: project.id } },
                    body,
                })
                navigate(`/projects/${project.id}/actions`)
            } else {
                await oapiClient.PATCH("/api/admin/projects/{projectID}/actions/{actionID}", {
                    params: {
                        path: { projectID: project.id, actionID: entityId! },
                    },
                    body,
                })
            }
        } finally {
            setIsSaving(false)
        }
    }

    const onTest = async () => {
        if (!selectedType) return
        setIsTesting(true)
        setTestResult(null)
        try {
            const formData = form.getValues()
            const body: Record<string, unknown> = {
                type: selectedType,
                config: formData.config ?? {},
                payload: formData.payload ?? {},
                variables: variablesToMap(formData.variables),
            }
            // Include action_id only for saved actions.
            if (entityId && !isNew) {
                body.action_id = entityId
            }
            const { data, error } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/actions/test",
                {
                    params: { path: { projectID: project.id } },
                    body: body as any,
                },
            )
            if (data) {
                setTestResult(data)
                setActiveTab("results")
            } else {
                setTestResult({
                    status: "error",
                    error: error?.detail ?? "Unknown error",
                })
                setActiveTab("results")
            }
        } catch (e) {
            setTestResult({
                status: "error",
                error: e instanceof Error ? e.message : "Request failed",
            })
            setActiveTab("results")
        } finally {
            setIsTesting(false)
        }
    }

    if (!isNew && !existingAction) return null

    return (
        <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 overflow-hidden">
            {/* Left: Form (scrollable) */}
            <div className="h-full overflow-y-auto w-2/5 bg-background p-8">
                <div className="space-y-6">
                    {/* Header */}
                    <div className="flex items-center gap-3">
                        <Button
                            variant="ghost"
                            size="icon"
                            type="button"
                            onClick={() => navigate(`/projects/${project.id}/actions`)}
                        >
                            <ArrowLeft className="h-4 w-4" />
                        </Button>
                        <h1 className="text-2xl font-semibold tracking-tight">
                            {isNew
                                ? t("create_action", "Create Action")
                                : t("edit_action", "Edit Action")}
                        </h1>
                        {selectedMeta && (
                            <Badge variant="secondary">{selectedMeta.name}</Badge>
                        )}
                        {!selectedMeta && selectedType && (
                            <Badge variant="secondary">{snakeToTitle(selectedType)}</Badge>
                        )}
                    </div>

                    {/* Name */}
                    <FieldGroup>
                        <Controller
                            name="name"
                            control={form.control}
                            render={({ field, fieldState }) => (
                                <Field data-invalid={fieldState.invalid} className="gap-2">
                                    <FieldLabel>
                                        {t("name")}{" "}
                                        <span className="inline text-destructive">*</span>
                                    </FieldLabel>
                                    <Input
                                        {...field}
                                        aria-invalid={fieldState.invalid}
                                        placeholder={t(
                                            "action_name_placeholder",
                                            "My Action",
                                        )}
                                        autoComplete="off"
                                    />
                                    {fieldState.invalid && (
                                        <FieldError errors={[fieldState.error]} />
                                    )}
                                </Field>
                            )}
                        />
                    </FieldGroup>

                    {/* Variables — user-defined name + binding rows */}
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
                                                <Select value={typeField.value} onValueChange={typeField.onChange}>
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

                    {/* Config section (from config_schema) */}
                    {selectedMeta?.config_schema && (
                        <div className="border-t pt-6">
                            <SchemaFields
                                title="Configuration"
                                description={selectedMeta.description}
                                parent="config"
                                schema={selectedMeta.config_schema as Schema}
                                form={form}
                                variableNames={variableNames}
                            />
                        </div>
                    )}

                    {/* Payload section (from payload_schema) */}
                    {selectedMeta?.payload_schema && (
                        <div className="border-t pt-6">
                            <SchemaFields
                                title="Payload"
                                parent="payload"
                                schema={selectedMeta.payload_schema as Schema}
                                form={form}
                                variableNames={variableNames}
                            />
                        </div>
                    )}

                    {/* Actions */}
                    <div className="flex items-center gap-3 border-t pt-6">
                        <Button type="submit" disabled={isSaving || !selectedType}>
                            {isSaving
                                ? t("saving", "Saving...")
                                : isNew
                                  ? t("create_action", "Create Action")
                                  : t("save_action", "Save Action")}
                        </Button>
                        <Button
                            type="button"
                            variant="outline"
                            disabled={isTesting || !selectedType}
                            onClick={onTest}
                        >
                            {isTesting ? t("testing", "Testing...") : t("test", "Test")}
                        </Button>
                        <Button
                            type="button"
                            variant="outline"
                            onClick={() => navigate(`/projects/${project.id}/actions`)}
                        >
                            {t("cancel")}
                        </Button>
                    </div>
                </div>
            </div>

            {/* Right: Preview / Results tabs */}
            <div className="flex flex-col w-3/5 border-l overflow-hidden">
                <nav className="flex gap-1 px-8 pt-8 border-b bg-card/50">
                    {([
                        { key: "preview", label: t("preview", "Preview") },
                        { key: "results", label: t("results", "Results") },
                    ] as const).map((tab) => {
                        const isDisabled = tab.key === "results" && !testResult
                        const tabButton = (
                            <button
                                key={tab.key}
                                type="button"
                                disabled={isDisabled}
                                onClick={() => setActiveTab(tab.key)}
                                className={cn(
                                    "flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-t-lg border-b-2 transition-colors -mb-px",
                                    activeTab === tab.key
                                        ? "border-primary text-foreground bg-background"
                                        : "border-transparent text-muted-foreground hover:text-foreground hover:bg-muted/50",
                                    isDisabled && "opacity-50 pointer-events-none",
                                )}
                            >
                                {tab.label}
                            </button>
                        )

                        if (isDisabled) {
                            return (
                                <Tooltip key={tab.key}>
                                    <TooltipTrigger asChild>
                                        <span className="cursor-default">{tabButton}</span>
                                    </TooltipTrigger>
                                    <TooltipContent side="bottom">
                                        {t("test_to_see_results", "Run a test to see results")}
                                    </TooltipContent>
                                </Tooltip>
                            )
                        }

                        return tabButton
                    })}
                </nav>

                <div className="flex-1 overflow-y-auto p-8">
                    {activeTab === "preview" && (
                        previewHtml ? (
                            <iframe
                                ref={iframeRef}
                                srcDoc={previewHtml}
                                title="Action Preview"
                                className="w-full rounded-lg bg-background"
                                style={{ height: iframeHeight, border: "none" }}
                                sandbox="allow-scripts"
                                onLoad={handleIframeLoad}
                            />
                        ) : (
                            <div className="flex items-center justify-center h-48 border rounded-lg text-muted-foreground text-sm">
                                {t("no_preview", "No preview available")}
                            </div>
                        )
                    )}

                    {activeTab === "results" && testResult && (
                        <TestResultPanel result={testResult} />
                    )}
                </div>
            </div>
        </form>
    )
}
