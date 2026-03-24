import { useCallback, useContext, useEffect, useMemo, useState } from "react"
import type { FieldPath } from "react-hook-form"
import { Controller, useForm } from "react-hook-form"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { ArrowLeft } from "lucide-react"

import oapiClient from "@/oapi/client"
import type { Provider, ProviderMeta, CreateProvider } from "@/oapi/client"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import type { SchemaProperty, Schema } from "@/components/schema-fields"

import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Field, FieldLabel } from "@/components/ui/field"
import { FormSchemaFields } from "@/components/schema-fields"
import { StaggeredMosaic } from "@/components/icon-mosaic"

import { SenderIdentityList } from "@/components/sender-identity-list"

type ProviderWithExtras = Provider & {
    setup?: { name: string; value: string }[]
    external_id?: string
}

type ProviderFormValues = CreateProvider & { module: string }

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

    const form = useForm<ProviderFormValues>({
        values: provider
            ? {
                  name: provider.name,
                  data: provider.data,
                  module: effectiveModule ?? "",
                  link_wrap: provider?.link_wrap ?? false,
              }
            : {
                  name: "",
                  data: {},
                  module: effectiveModule ?? "",
                  link_wrap: true,
              },
    })

    const handleSubmit = async (values: ProviderFormValues) => {
        if (isExternal) return
        if (!effectiveModule) return
        setIsSaving(true)
        try {
            const { name, data, link_wrap } = values
            const body = { name, data, link_wrap }
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
