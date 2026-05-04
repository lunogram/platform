import { useCallback, useContext, useEffect, useMemo, useState } from "react"
import type { FieldPath } from "react-hook-form"
import { Controller, useForm } from "react-hook-form"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { ArrowLeft, Gauge } from "lucide-react"

import oapiClient from "@/oapi/client"
import type {
    Action,
    ActionMeta,
    CreateAction,
    CreateProvider,
    Provider,
    ProviderMeta,
    UpdateAction,
    UpdateProvider,
} from "@/oapi/client"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import type { SchemaProperty, Schema } from "@/components/schema-fields"

import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Field, FieldLabel, FieldDescription } from "@/components/ui/field"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { FormSchemaFields } from "@/components/schema-fields"
import { StaggeredMosaic } from "@/components/icon-mosaic"

import { SenderIdentityList } from "@/components/sender-identity-list"

type IntegrationKind = "provider" | "action"

type ProviderWithExtras = Provider & {
    setup?: { name: string; value: string }[]
    external_id?: string
}

type ActionWithExtras = Action

type IntegrationMeta =
    | { kind: "provider"; meta: ProviderMeta }
    | { kind: "action"; meta: ActionMeta }

type IntegrationFormValues = {
    kind: IntegrationKind
    module: string
    name: string
    data?: Record<string, unknown>
    config?: Record<string, unknown>
    link_wrap?: boolean
    rate_limit?: number | null
    rate_interval?: string | null
}

/** Map human-friendly interval labels to Go duration strings */
const RATE_INTERVAL_OPTIONS = [
    { value: "1s", label: "per second" },
    { value: "1m", label: "per minute" },
    { value: "1h", label: "per hour" },
    { value: "24h", label: "per day" },
] as const

function normalizeKind(kind?: string, id?: string): IntegrationKind {
    if (kind === "action" || kind === "provider") return kind
    if (id) return "provider"
    return "provider"
}

/**
 * Handles both creating and editing integrations:
 * - Create provider: /projects/:projectId/integrations/new/provider/:module
 * - Create action:   /projects/:projectId/integrations/new/action/:module
 * - Edit provider:   /projects/:projectId/integrations/provider/:id
 * - Edit action:     /projects/:projectId/integrations/action/:id
 *
 * Legacy routes still supported:
 * - /projects/:projectId/integrations/new/:module (provider)
 * - /projects/:projectId/integrations/:id (provider)
 */
export default function IntegrationSetup() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { kind: kindParam, module: moduleName, id } = useParams()

    const kind = normalizeKind(kindParam, id)
    const isEdit = !!id && id !== "new"

    const [provider, setProvider] = useState<ProviderWithExtras | undefined>()
    const [action, setAction] = useState<ActionWithExtras | undefined>()
    const [isSaving, setIsSaving] = useState(false)
    const [listKey] = useState(0)

    const [providerOptions] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/providers/meta",
                {
                    params: { path: { projectID: project.id } },
                },
            )
            return data ?? []
        }, [project.id]),
    )

    const [actionOptions] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                params: { path: { projectID: project.id } },
            })
            return data ?? []
        }, [project.id]),
    )

    useEffect(() => {
        if (!isEdit || !id) return

        if (kind === "action") {
            oapiClient
                .GET("/api/admin/projects/{projectID}/actions/{actionID}", {
                    params: {
                        path: {
                            projectID: project.id,
                            actionID: id,
                        },
                    },
                })
                .then(({ data }) => {
                    if (!data) {
                        navigate(`/projects/${project.id}/integrations`)
                        return
                    }
                    setAction(data)
                })
                .catch(() => navigate(`/projects/${project.id}/integrations`))
            return
        }

        oapiClient
            .GET("/api/admin/projects/{projectID}/providers", {
                params: {
                    path: { projectID: project.id },
                    query: { limit: 100 },
                },
            })
            .then(({ data }) => {
                const found = data?.results?.find((p) => p.id === id)
                if (!found) {
                    navigate(`/projects/${project.id}/integrations`)
                    return
                }

                oapiClient
                    .GET("/api/admin/projects/{projectID}/providers/{type}/{providerID}", {
                        params: {
                            path: {
                                projectID: project.id,
                                type: found.module,
                                providerID: found.id,
                            },
                        },
                    })
                    .then(({ data: full }) => setProvider((full as ProviderWithExtras) ?? found))
                    .catch(() => setProvider(found))
            })
            .catch(() => navigate(`/projects/${project.id}/integrations`))
    }, [isEdit, id, kind, project.id, navigate])

    const activeMeta = useMemo<IntegrationMeta | undefined>(() => {
        if (kind === "action") {
            if (!actionOptions) return undefined
            const targetType = isEdit ? action?.type : moduleName
            const meta = actionOptions.find((o: ActionMeta) => o.type === targetType)
            return meta ? { kind: "action", meta } : undefined
        }

        if (!providerOptions) return undefined
        const targetType = isEdit ? provider?.module : moduleName
        const meta = providerOptions.find((o: ProviderMeta) => o.type === targetType)
        return meta ? { kind: "provider", meta } : undefined
    }, [actionOptions, providerOptions, kind, isEdit, action, provider, moduleName])

    const effectiveModule = useMemo(() => {
        if (activeMeta?.kind === "action") return activeMeta.meta.type
        if (activeMeta?.kind === "provider") return activeMeta.meta.type
        return undefined
    }, [activeMeta])

    const isExternal = !!provider?.external_id

    const dataSchema = useMemo((): Schema | undefined => {
        if (!activeMeta) return undefined
        if (activeMeta.kind === "action") {
            return (activeMeta.meta.config_schema as unknown as Schema | undefined) ?? undefined
        }

        const schema = activeMeta.meta.schema as unknown as Schema | undefined
        if (!schema?.properties) return undefined

        const rawSchema = Array.isArray(schema.properties)
            ? schema.properties.find((p) => p.name === "data")?.schema
            : (schema.properties as Record<string, Schema>).data
        if (!rawSchema?.properties) return rawSchema

        const senderKeys = new Set(["default_from"])
        const props: SchemaProperty[] = Array.isArray(rawSchema.properties)
            ? rawSchema.properties
            : Object.entries(rawSchema.properties as Record<string, Schema>).map(
                  ([name, propSchema]) => ({
                      name,
                      schema: propSchema,
                  }),
              )

        const filtered = props.filter((p) => !senderKeys.has(p.name))

        return { ...rawSchema, properties: filtered }
    }, [activeMeta])

    const effectiveChannels =
        activeMeta?.kind === "provider"
            ? isEdit
                ? provider?.channels
                : activeMeta.meta.channels
            : undefined

    const senderIdentityChannel = effectiveChannels?.find((c) => c === "email" || c === "sms")

    const rateLimitOverride =
        activeMeta?.kind === "provider" && activeMeta.meta.rate_limit?.override === true
    const manifestRateLimit =
        activeMeta?.kind === "provider" ? activeMeta.meta.rate_limit : undefined
    const maxRateLimit =
        activeMeta?.kind === "provider" ? activeMeta.meta.max_rate_limit : undefined

    const form = useForm<IntegrationFormValues>({
        values:
            kind === "action"
                ? {
                      kind: "action",
                      name: action?.name ?? "",
                      module: effectiveModule ?? "",
                      config: action?.config ?? {},
                  }
                : provider
                  ? {
                        kind: "provider",
                        name: provider.name,
                        data: provider.data,
                        module: effectiveModule ?? "",
                        link_wrap: provider?.link_wrap ?? false,
                        rate_limit: provider?.rate_limit ?? null,
                        rate_interval: provider?.rate_interval ?? "1s",
                    }
                  : {
                        kind: "provider",
                        name: "",
                        data: {},
                        module: effectiveModule ?? "",
                        link_wrap: true,
                        rate_limit: null,
                        rate_interval: "1s",
                    },
    })

    const handleSubmit = async (values: IntegrationFormValues) => {
        if (isExternal) return
        if (!effectiveModule) return

        setIsSaving(true)
        try {
            if (kind === "action") {
                const body: CreateAction & UpdateAction = {
                    name: values.name,
                    type: effectiveModule,
                    config: values.config,
                }

                if (isEdit && action?.id) {
                    await oapiClient.PATCH("/api/admin/projects/{projectID}/actions/{actionID}", {
                        params: {
                            path: {
                                projectID: project.id,
                                actionID: action.id,
                            },
                        },
                        body,
                    })
                } else {
                    const { data: created } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/actions",
                        {
                            params: { path: { projectID: project.id } },
                            body,
                        },
                    )
                    if (created) {
                        navigate(`/projects/${project.id}/integrations/action/${created.id}`)
                        return
                    }
                }

                navigate(`/projects/${project.id}/integrations`)
                return
            }

            const body: CreateProvider & UpdateProvider = {
                name: values.name,
                data: values.data,
                link_wrap: values.link_wrap,
            }

            if (rateLimitOverride) {
                body.rate_limit =
                    values.rate_limit && values.rate_limit > 0 ? values.rate_limit : null
                body.rate_interval =
                    values.rate_limit && values.rate_limit > 0
                        ? (values.rate_interval ?? "1s")
                        : null
            }

            if (isEdit && provider?.id) {
                await oapiClient.PATCH(
                    "/api/admin/projects/{projectID}/providers/{type}/{providerID}",
                    {
                        params: {
                            path: {
                                projectID: project.id,
                                type: effectiveModule,
                                providerID: provider.id,
                            },
                        },
                        body,
                    },
                )
            } else {
                const { data: created } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/providers/{type}",
                    {
                        params: {
                            path: {
                                projectID: project.id,
                                type: effectiveModule,
                            },
                        },
                        body,
                    },
                )

                if (created) {
                    navigate(`/projects/${project.id}/integrations/provider/${created.id}`)
                    return
                }
            }

            navigate(`/projects/${project.id}/integrations`)
        } finally {
            setIsSaving(false)
        }
    }

    const backUrl = isEdit
        ? `/projects/${project.id}/integrations`
        : `/projects/${project.id}/integrations/new`

    if (!activeMeta && providerOptions && actionOptions && !(isEdit && !provider && !action)) {
        navigate(backUrl)
        return null
    }

    if (!activeMeta) {
        return (
            <div className="flex items-center justify-center h-32 text-muted-foreground p-6">
                <p className="text-sm">{t("loading", "Loading...")}</p>
            </div>
        )
    }

    if (isEdit && kind === "provider" && !provider) {
        return (
            <div className="flex items-center justify-center h-32 text-muted-foreground p-6">
                <p className="text-sm">{t("loading", "Loading...")}</p>
            </div>
        )
    }

    if (isEdit && kind === "action" && !action) {
        return (
            <div className="flex items-center justify-center h-32 text-muted-foreground p-6">
                <p className="text-sm">{t("loading", "Loading...")}</p>
            </div>
        )
    }

    const showSenderIdentity =
        kind === "provider" &&
        (senderIdentityChannel === "email" || senderIdentityChannel === "sms")

    const title =
        isEdit && kind === "action"
            ? action?.name
            : isEdit && kind === "provider"
              ? provider?.name
              : activeMeta.meta.name

    const subtitle =
        isEdit && kind === "action"
            ? activeMeta.meta.name
            : isEdit && kind === "provider"
              ? activeMeta.meta.name
              : kind === "action"
                ? t(
                      "integration_setup_action_hint",
                      "Configure this action integration to run functions in journeys.",
                  )
                : t(
                      "integration_setup_hint",
                      "Fill out the fields below to connect this integration.",
                  )

    return (
        <div className="flex flex-col min-h-full">
            <div className="border-b bg-card/50 relative overflow-hidden">
                <div
                    className="absolute top-1/2 -translate-y-1/2 left-[50%] xl:left-[30%] right-0 hidden lg:block pointer-events-none opacity-[0.8]"
                    style={{
                        maskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                        WebkitMaskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                    }}
                >
                    <StaggeredMosaic
                        provider={{
                            id: activeMeta.meta.type,
                            name: activeMeta.meta.name,
                            icon: activeMeta.meta.icon,
                            color: activeMeta.meta.color,
                        }}
                        cols={12}
                        rows={4}
                    />
                </div>

                <div className="p-4 sm:p-6 py-8 sm:py-10 relative z-20">
                    <div className="flex items-center gap-3">
                        <Button
                            variant="ghost"
                            size="icon"
                            type="button"
                            onClick={() => navigate(backUrl)}
                        >
                            <ArrowLeft className="h-4 w-4" />
                        </Button>
                        <div className="flex items-center gap-3">
                            <div className="space-y-0.5">
                                <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
                                <p className="text-sm text-muted-foreground">{subtitle}</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <div className="flex-1 overflow-y-auto p-6">
                <form
                    id="integration-form"
                    onSubmit={form.handleSubmit(handleSubmit)}
                    className="grid gap-6 max-w-2xl"
                >
                    {kind === "provider" &&
                        isEdit &&
                        provider?.setup &&
                        provider.setup.length > 0 && (
                            <>
                                <h4 className="text-sm font-medium">{t("details", "Details")}</h4>
                                {provider.setup.map((item) => (
                                    <Field key={item.name}>
                                        <FieldLabel className="text-muted-foreground">
                                            {item.name}
                                        </FieldLabel>
                                        <Input value={item.value} disabled />
                                    </Field>
                                ))}
                                <Separator />
                            </>
                        )}

                    <Field>
                        <FieldLabel>
                            {t("name")} <span className="text-destructive">*</span>
                        </FieldLabel>
                        <Input
                            {...form.register("name", { required: true })}
                            disabled={kind === "provider" && isExternal}
                        />
                    </Field>

                    {kind === "provider" && !isExternal && dataSchema && (
                        <FormSchemaFields parent="data" schema={dataSchema} form={form} />
                    )}

                    {kind === "action" && dataSchema && (
                        <FormSchemaFields parent="config" schema={dataSchema} form={form} />
                    )}

                    {kind === "provider" && !isExternal && (
                        <Controller
                            control={form.control}
                            name="link_wrap"
                            render={({ field }) => (
                                <div className="flex items-center justify-between gap-4 rounded-lg border p-4">
                                    <div className="space-y-0.5">
                                        <Label htmlFor="link_wrap" className="text-sm font-medium">
                                            {t("link_wrapping", "Link Wrapping")}
                                        </Label>
                                        <p className="text-xs text-muted-foreground">
                                            {t(
                                                "link_wrapping_description",
                                                "Wrap links in messages to track clicks.",
                                            )}
                                        </p>
                                    </div>
                                    <Switch
                                        id="link_wrap"
                                        checked={!!field.value}
                                        onCheckedChange={field.onChange}
                                    />
                                </div>
                            )}
                        />
                    )}

                    {kind === "provider" && !isExternal && manifestRateLimit && (
                        <>
                            <Separator />
                            <div className="space-y-4">
                                <div className="flex items-center gap-2">
                                    <Gauge className="h-4 w-4 text-muted-foreground" />
                                    <h4 className="text-sm font-medium">
                                        {t("rate_limit", "Rate Limit")}
                                    </h4>
                                </div>

                                <div className="rounded-lg border p-4 space-y-4">
                                    <div className="space-y-1">
                                        <p className="text-sm text-muted-foreground">
                                            {t(
                                                "rate_limit_default_label",
                                                "Default: {{limit}} requests / {{interval}}",
                                                {
                                                    limit: manifestRateLimit.limit,
                                                    interval: manifestRateLimit.interval || "1s",
                                                },
                                            )}
                                        </p>
                                        {!rateLimitOverride && (
                                            <p className="text-xs text-muted-foreground/70">
                                                {t(
                                                    "rate_limit_locked",
                                                    "This provider does not allow rate limit adjustments.",
                                                )}
                                            </p>
                                        )}
                                    </div>

                                    {rateLimitOverride && (
                                        <div className="space-y-3">
                                            <Field>
                                                <FieldLabel>
                                                    {t("rate_limit_override", "Custom Rate Limit")}
                                                </FieldLabel>
                                                <FieldDescription>
                                                    {t(
                                                        "rate_limit_override_description",
                                                        "Override the default rate limit for this provider. Leave empty to use the default." +
                                                            (maxRateLimit
                                                                ? ` Maximum: ${maxRateLimit} requests per minute.`
                                                                : ""),
                                                    )}
                                                </FieldDescription>
                                                <div className="flex items-center gap-2">
                                                    <Input
                                                        type="number"
                                                        min={0}
                                                        max={maxRateLimit ?? undefined}
                                                        placeholder={String(
                                                            manifestRateLimit.limit,
                                                        )}
                                                        className="w-28"
                                                        {...form.register("rate_limit", {
                                                            valueAsNumber: true,
                                                            min: 0,
                                                            max: maxRateLimit ?? undefined,
                                                        })}
                                                    />
                                                    <Controller
                                                        control={form.control}
                                                        name="rate_interval"
                                                        render={({ field }) => (
                                                            <Select
                                                                value={field.value ?? "1s"}
                                                                onValueChange={field.onChange}
                                                            >
                                                                <SelectTrigger className="w-36">
                                                                    <SelectValue />
                                                                </SelectTrigger>
                                                                <SelectContent>
                                                                    {RATE_INTERVAL_OPTIONS.map(
                                                                        (opt) => (
                                                                            <SelectItem
                                                                                key={opt.value}
                                                                                value={opt.value}
                                                                            >
                                                                                {opt.label}
                                                                            </SelectItem>
                                                                        ),
                                                                    )}
                                                                </SelectContent>
                                                            </Select>
                                                        )}
                                                    />
                                                </div>
                                            </Field>
                                        </div>
                                    )}
                                </div>
                            </div>
                        </>
                    )}
                </form>

                {kind === "provider" && isEdit && provider?.id && showSenderIdentity && (
                    <div className="max-w-2xl mt-8">
                        <SenderIdentityList
                            key={listKey}
                            projectId={project.id}
                            providerId={provider.id}
                            channel={senderIdentityChannel as "email" | "sms"}
                            defaultFromId={
                                form.watch(
                                    "data.default_from" as FieldPath<IntegrationFormValues>,
                                ) as string | undefined
                            }
                            onDefaultChange={(identityId) =>
                                form.setValue(
                                    "data.default_from" as FieldPath<IntegrationFormValues>,
                                    identityId,
                                    {
                                        shouldDirty: true,
                                    },
                                )
                            }
                        />
                    </div>
                )}

                {(kind !== "provider" || !isExternal) && (
                    <div className="flex items-center gap-3 pt-8 pb-6 max-w-2xl">
                        <Button type="submit" form="integration-form" disabled={isSaving}>
                            {isSaving
                                ? t("saving", "Saving...")
                                : isEdit
                                  ? t("update_integration", "Update Integration")
                                  : t("create_integration", "Create Integration")}
                        </Button>
                        <Button type="button" variant="outline" onClick={() => navigate(backUrl)}>
                            {t("cancel")}
                        </Button>
                    </div>
                )}
            </div>
        </div>
    )
}
