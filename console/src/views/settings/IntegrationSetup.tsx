import { useCallback, useContext, useEffect, useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { ArrowLeft } from "lucide-react"

import api from "../../api"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import type { Provider, ProviderCreateParams, ProviderMeta } from "../../types"
import type { SchemaProperty } from "@/components/schema-fields"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Field, FieldLabel } from "@/components/ui/field"
import { FormSchemaFields } from "@/components/schema-fields"
import { StaggeredMosaic } from "@/components/icon-mosaic"

import { SenderIdentityList } from "@/components/sender-identity-list"

/**
 * Handles both creating and editing integrations:
 * - Create: /projects/:projectId/integrations/new/:channel/:module
 * - Edit:   /projects/:projectId/integrations/:id
 */
export default function IntegrationSetup() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { channel, module: moduleName, id } = useParams()
    const isEdit = !!id && id !== "new"

    const [provider, setProvider] = useState<Provider | undefined>()
    const [isSaving, setIsSaving] = useState(false)
    const [listKey] = useState(0)

    // Load all provider metas to find the schema
    const [options] = useResolver(
        useCallback(async () => await api.providers.options(project.id), [project]),
    )

    // For edit mode: load the existing provider
    useEffect(() => {
        if (isEdit && id) {
            api.providers
                .search(project.id, { limit: 100 } as any)
                .then((result) => {
                    const found = result?.results?.find((p: Provider) => p.id === id)
                    if (found) {
                        // Fetch full provider details
                        api.providers
                            .get(project.id, found.channel, found.module, found.id)
                            .then((full) => setProvider(full))
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
            return options.find(
                (o: ProviderMeta) => o.group === provider.channel && o.type === provider.module,
            )
        }
        return options.find((o: ProviderMeta) => o.group === channel && o.type === moduleName)
    }, [options, isEdit, provider, channel, moduleName])

    const effectiveChannel = isEdit ? provider?.channel : channel
    const effectiveModule = isEdit ? provider?.module : moduleName

    // Strip any legacy default_from* fields from the schema so they aren't
    // rendered as generic form inputs — sender identity is handled by a
    // dedicated component based on the channel.
    const dataSchema = useMemo(() => {
        const rawSchema = Array.isArray(meta?.schema?.properties)
            ? meta?.schema?.properties?.find((p: SchemaProperty) => p.name === "data")?.schema
            : meta?.schema?.properties?.data
        if (!rawSchema?.properties) return rawSchema

        const senderKeys = new Set(["default_from"])
        const props = Array.isArray(rawSchema.properties)
            ? rawSchema.properties
            : Object.entries(rawSchema.properties).map(([name, schema]: [string, any]) => ({
                  name,
                  schema,
              }))

        const filtered = props.filter((p: any) => !senderKeys.has(p.name))

        return { ...rawSchema, properties: filtered }
    }, [meta])

    const senderIdentityChannel = effectiveChannel === "text" ? "sms" : effectiveChannel

    const form = useForm<ProviderCreateParams>({
        values: provider
            ? {
                  name: provider.name,
                  data: provider.data,
                  module: effectiveModule ?? "",
                  channel: effectiveChannel ?? "",
              }
            : {
                  name: "",
                  data: {},
                  module: effectiveModule ?? "",
                  channel: effectiveChannel ?? "",
              },
    })

    const handleSubmit = async (values: ProviderCreateParams) => {
        if (!effectiveChannel || !effectiveModule) return
        setIsSaving(true)
        try {
            const params = { ...values, module: effectiveModule, channel: effectiveChannel }
            if (isEdit && provider?.id) {
                await api.providers.update(project.id, provider.id, params)
            } else {
                const created = await api.providers.create(project.id, params)
                navigate(`/projects/${project.id}/integrations/${created.id}`)
                return
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
                    {isEdit && provider?.setup?.length > 0 && (
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
                        <Input {...form.register("name", { required: true })} />
                    </Field>

                    <FormSchemaFields parent="data" schema={dataSchema} form={form} />
                </form>

                {/* Sender identity management — edit mode only */}
                {isEdit && provider?.id && showSenderIdentity && (
                    <div className="max-w-2xl mt-8">
                        <SenderIdentityList
                            key={listKey}
                            projectId={project.id}
                            providerId={provider.id}
                            channel={senderIdentityChannel as "email" | "sms"}
                            defaultFromId={form.watch("data.default_from")}
                            onDefaultChange={(identityId) =>
                                form.setValue("data.default_from", identityId, {
                                    shouldDirty: true,
                                })
                            }
                        />
                    </div>
                )}

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
            </div>
        </div>
    )
}
