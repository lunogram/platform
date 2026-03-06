import { Controller, useForm, useWatch } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams } from "react-router"
import { useCallback, useContext, useEffect, useMemo, useState } from "react"
import { ArrowLeft } from "lucide-react"

import oapiClient, { type Action, type ActionMeta, type TestActionResult } from "@/oapi/client"
import { ProjectContext } from "@/contexts"
import { useResolver } from "@/hooks"

import { SchemaFields, type Schema } from "@/components/schema-fields"

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { snakeToTitle } from "@/utils"

import {
    actionFormSchema,
    storedToForm,
    variablesToMap,
    type ActionFormValues,
    type StoredVariable,
} from "./action-form-types"
import { ActionPreviewPanel } from "./ActionPreviewPanel"
import { TestVariablesDialog } from "./TestVariablesDialog"
import { VariablesSection } from "./VariablesSection"

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
    const [showTestModal, setShowTestModal] = useState(false)
    const [testVariableValues, setTestVariableValues] = useState<Record<string, string>>({})
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

    // Watch form values for live preview updates
    const config = useWatch({ control: form.control, name: "config" })
    const payload = useWatch({ control: form.control, name: "payload" })
    const variables = useWatch({ control: form.control, name: "variables" })

    // Derive variable names for autocomplete in payload fields
    const variableNames = useMemo(
        () => (variables ?? []).map((v) => v.name.trim()).filter(Boolean),
        [variables],
    )

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

    /** Open the test variables modal if variables exist, otherwise run test directly */
    const onTestClick = () => {
        const formData = form.getValues()
        const vars = (formData.variables ?? []).filter((v) => v.name.trim())
        if (vars.length > 0) {
            // Pre-populate modal with default values
            const defaults: Record<string, string> = {}
            for (const v of vars) {
                defaults[v.name.trim()] = v.default
            }
            setTestVariableValues(defaults)
            setShowTestModal(true)
        } else {
            executeTest()
        }
    }

    /** Execute the test action with the given variable overrides */
    const executeTest = async (overrides?: Record<string, string>) => {
        if (!selectedType) return
        setIsTesting(true)
        setTestResult(null)
        setActiveTab("results")
        try {
            const formData = form.getValues()
            const body: Record<string, unknown> = {
                type: selectedType,
                config: formData.config ?? {},
                payload: formData.payload ?? {},
                variables: variablesToMap(formData.variables, overrides),
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
            } else {
                setTestResult({
                    status: "error",
                    error: error?.detail ?? "Unknown error",
                })
            }
        } catch (e) {
            setTestResult({
                status: "error",
                error: e instanceof Error ? e.message : "Request failed",
            })
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
                        {selectedMeta && <Badge variant="secondary">{selectedMeta.name}</Badge>}
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
                                        placeholder={t("action_name_placeholder", "My Action")}
                                        autoComplete="off"
                                    />
                                    {fieldState.invalid && (
                                        <FieldError errors={[fieldState.error]} />
                                    )}
                                </Field>
                            )}
                        />
                    </FieldGroup>

                    {/* Variables */}
                    <VariablesSection form={form} />

                    {/* Config section (from config_schema) */}
                    {selectedMeta?.config_schema && (
                        <div className="border-t pt-6">
                            <SchemaFields
                                title="Configuration"
                                description={selectedMeta.description}
                                parent="config"
                                schema={selectedMeta.config_schema as unknown as Schema}
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
                                schema={selectedMeta.payload_schema as unknown as Schema}
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
                            isLoading={isTesting}
                            onClick={onTestClick}
                        >
                            {t("test", "Test")}
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
            <ActionPreviewPanel
                selectedType={selectedType}
                projectId={project.id}
                isTesting={isTesting}
                testResult={testResult}
                activeTab={activeTab}
                setActiveTab={setActiveTab}
                config={config}
                payload={payload}
                variables={variables}
            />

            {/* Test Variables Modal */}
            <TestVariablesDialog
                open={showTestModal}
                onOpenChange={setShowTestModal}
                form={form}
                testVariableValues={testVariableValues}
                setTestVariableValues={setTestVariableValues}
                isTesting={isTesting}
                onRunTest={executeTest}
            />
        </form>
    )
}
