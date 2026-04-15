import { useCallback, useContext, useEffect, useMemo, useState } from "react"
import type { FieldPath } from "react-hook-form"
import { Controller, useForm } from "react-hook-form"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { ArrowLeft, Gauge } from "lucide-react"

import oapiClient from "@/oapi/client"
import type { Provider, ProviderMeta, CreateProvider, UpdateProvider } from "@/oapi/client"
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

type ProviderWithExtras = Provider & {
    setup?: { name: string; value: string }[]
    external_id?: string
}

type ProviderFormValues = {
    name: string
    data: Record<string, unknown>
    module: string
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

/**
 * Handles both creating and editing integrations:
 * - Create: /projects/:projectId/integrations/new/:channel/:module
 * - Edit:   /projects/:projectId/integrations/:id
 *
 * External integrations (with external_id) are shown as read-only.
 * Email-channel integrations also show a domain management section (enterprise only).
 */
export default function IntegrationSetup() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { module: moduleName, id } = useParams()
    const isEdit = !!id && id !== "new"

    const [provider, setProvider] = useState<ProviderWithExtras | undefined>()
    const [isSaving, setIsSaving] = useState(false)
    const [listKey] = useState(0)

    // Load all provider metas to find the schema
    const [options] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/providers/meta",
                {
                    params: { path: { projectID: project.id } },
                },
            )
            return data
        }, [project]),
    )

    // For edit mode: load the existing provider
    useEffect(() => {
        if (isEdit && id) {
            oapiClient
                .GET("/api/admin/projects/{projectID}/providers", {
                    params: {
                        path: { projectID: project.id },
                        query: { limit: 100 },
                    },
                })
                .then(({ data }) => {
                    const found = data?.results?.find((p) => p.id === id)
                    if (found) {
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
                            .then(({ data: full }) => setProvider(full ?? found))
                            .catch(() => setProvider(found))
                    } else {
                        navigate(`/projects/${project.id}/integrations`)
                    }
                })
                .catch(() => navigate(`/projects/${project.id}/integrations`))
        }
    }, [isEdit, id, project.id, navigate])

    // Resolve the active meta
    const meta = useMemo(() => {
        if (!options) return undefined
        if (isEdit && provider) {
            return options.find((o: ProviderMeta) => o.type === provider.module)
        }
        return options.find((o: ProviderMeta) => o.type === moduleName)
    }, [options, isEdit, provider, moduleName])

    const effectiveChannels = isEdit ? provider?.channels : meta?.channels
    const effectiveModule = isEdit ? provider?.module : moduleName
    const isExternal = !!provider?.external_id

    // Strip any legacy default_from* fields from the schema so they aren't
    // rendered as generic form inputs — sender identity is handled by a
    // dedicated component based on the channel.
    const dataSchema = useMemo((): Schema | undefined => {
        const schema = meta?.schema as unknown as Schema | undefined
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
    }, [meta])

    const senderIdentityChannel = effectiveChannels?.find((c) => c === "email" || c === "sms")

    const rateLimitOverride = meta?.rate_limit?.override === true
    const manifestRateLimit = meta?.rate_limit
    const maxRateLimit = meta?.max_rate_limit

    const form = useForm<ProviderFormValues>({
        values: provider
            ? {
                  name: provider.name,
                  data: (provider.data as Record<string, unknown>) ?? {},
                  module: effectiveModule ?? "",
                  link_wrap: provider?.link_wrap ?? false,
                  rate_limit: provider?.rate_limit?.limit ?? null,
                  rate_interval: provider?.rate_limit?.interval ?? "1s",
              }
            : {
                  name: "",
                  data: {},
                  module: effectiveModule ?? "",
                  link_wrap: true,
                  rate_limit: null,
                  rate_interval: "1s",
              },
    })

    const handleSubmit = async (values: ProviderFormValues) => {
        if (isExternal) return
        if (!effectiveModule) return
        setIsSaving(true)
        try {
            const { name, data, link_wrap, rate_limit, rate_interval } = values
            const body: CreateProvider & UpdateProvider = { name, data, link_wrap }

            // Only send rate limit fields when the manifest allows overrides
            if (rateLimitOverride) {
                const hasCustomRateLimit =
                    typeof rate_limit === "number" &&
                    Number.isFinite(rate_limit) &&
                    rate_limit > 0

                if (hasCustomRateLimit) {
                    body.rate_limit = {
                        limit: rate_limit,
                        interval: rate_interval ?? "1s",
                    }
                } else if (isEdit) {
                    // For edits, allow clearing an override back to module defaults.
                    body.rate_limit = {
                        limit: 0,
                        interval: manifestRateLimit?.interval ?? "1s",
                    }
                }
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
                    navigate(`/projects/${project.id}/integrations/${created.id}`)
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

    // Wait for data before rendering the form
    if (!meta && options && !(isEdit && !provider)) {
        // meta not found and not still loading provider — redirect back
        navigate(backUrl)
        return null
    }
    if (!meta) {
        return (
            <div className="flex items-center justify-center h-32 text-muted-foreground p-6">
                <p className="text-sm">{t("loading", "Loading...")}</p>
            </div>
        )
    }
    if (isEdit && !provider) {
        return (
            <div className="flex items-center justify-center h-32 text-muted-foreground p-6">
                <p className="text-sm">{t("loading", "Loading...")}</p>
            </div>
        )
    }

    const showSenderIdentity = senderIdentityChannel === "email" || senderIdentityChannel === "sms"

    return (
        <div className="flex flex-col min-h-full">
            {/* Header — ambient mosaic background (same pattern as UserDetail map) */}
            <div className="border-b bg-card/50 relative overflow-hidden">
                {/* Ambient mosaic — faded right-side background */}
                <div
                    className="absolute top-1/2 -translate-y-1/2 left-[50%] xl:left-[30%] right-0 hidden lg:block pointer-events-none opacity-[0.8]"
                    style={{
                        maskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                        WebkitMaskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                    }}
                >
                    <StaggeredMosaic
                        provider={
                            meta
                                ? {
                                      id: meta.type,
                                      name: meta.name,
                                      icon: meta.icon,
                                      color: meta.color,
                                  }
                                : undefined
                        }
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
                                <h1 className="text-2xl font-semibold tracking-tight">
                                    {isEdit ? provider?.name : meta.name}
                                </h1>
                                <p className="text-sm text-muted-foreground">
                                    {isEdit
                                        ? meta.name
                                        : t(
                                              "integration_setup_hint",
                                              "Fill out the fields below to connect this integration.",
                                          )}
                                </p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Form — full width below header */}
            <div className="flex-1 overflow-y-auto p-6">
                <form
                    id="integration-form"
                    onSubmit={form.handleSubmit(handleSubmit)}
                    className="grid gap-6 max-w-2xl"
                >
                    {isEdit && provider?.setup && provider.setup.length > 0 && (
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
                            disabled={isExternal}
                        />
                    </Field>

                    {!isExternal && dataSchema && (
                        <FormSchemaFields parent="data" schema={dataSchema} form={form} />
                    )}

                    {!isExternal && (
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

                    {/* Rate Limit Section */}
                    {!isExternal && manifestRateLimit && (
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

                {/* Sender identity management — edit mode only */}
                {isEdit && provider?.id && showSenderIdentity && (
                    <div className="max-w-2xl mt-8">
                        <SenderIdentityList
                            key={listKey}
                            projectId={project.id}
                            providerId={provider.id}
                            channel={senderIdentityChannel as "email" | "sms"}
                            defaultFromId={
                                form.watch("data.default_from" as FieldPath<ProviderFormValues>) as
                                    | string
                                    | undefined
                            }
                            onDefaultChange={(identityId) =>
                                form.setValue(
                                    "data.default_from" as FieldPath<ProviderFormValues>,
                                    identityId,
                                    {
                                        shouldDirty: true,
                                    },
                                )
                            }
                        />
                    </div>
                )}

                {!isExternal && (
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
