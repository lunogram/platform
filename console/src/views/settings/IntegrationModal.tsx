import { useCallback, useContext, useEffect, useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { providerFormSchema } from "@/validation/settings/integration-modal"
import { useTranslation } from "react-i18next"
import { ChevronLeft } from "lucide-react"
import oapiClient from "@/oapi/client"
import type { Provider, ProviderMeta, CreateProvider } from "@/oapi/client"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle } from "../../utils"
import type { Project } from "../../types"
import type { Schema } from "@/components/schema-fields"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Separator } from "@/components/ui/separator"
import { FormSchemaFields } from "@/components/schema-fields"

type ProviderWithExtras = Provider & {
    setup?: { name: string; value: string }[]
    external_id?: string
}

interface IntegrationFormParams {
    project: Project
    meta: ProviderMeta
    provider?: ProviderWithExtras
    onChange: (provider: Provider) => void
}

type ProviderFormValues = CreateProvider & { module: string }

function extractDataSchema(meta: ProviderMeta): Schema | undefined {
    const schema = meta.schema as unknown as Schema | undefined
    if (!schema?.properties) return undefined
    if (Array.isArray(schema.properties)) {
        return schema.properties.find((p) => p.name === "data")?.schema
    }
    return (schema.properties as Record<string, Schema>).data
}

export function IntegrationForm({
    project,
    provider: defaultProvider,
    onChange,
    meta,
}: IntegrationFormParams) {
    const { t } = useTranslation()
    const [provider, setProvider] = useState<ProviderWithExtras | undefined>(defaultProvider)
    const [isSaving, setIsSaving] = useState(false)

    const module = meta.type

    useEffect(() => {
        if (defaultProvider) {
            oapiClient
                .GET("/api/admin/projects/{projectID}/providers/{type}/{providerID}", {
                    params: {
                        path: {
                            projectID: project.id,
                            type: defaultProvider.module,
                            providerID: defaultProvider.id,
                        },
                    },
                })
                .then(({ data }) => {
                    if (data) setProvider(data as ProviderWithExtras)
                })
                .catch(() => {})
        }
    }, [project.id, defaultProvider])

    const form = useForm<ProviderFormValues>({
        resolver: zodResolver(providerFormSchema),
        values: provider
            ? {
                  name: provider.name,
                  data: provider.data,
                  module,
                  link_wrap: provider.link_wrap ?? true,
              }
            : { name: "", data: {}, module, link_wrap: true },
    })

    const handleSubmit = async (values: ProviderFormValues) => {
        setIsSaving(true)
        try {
            const { name, data, link_wrap } = values
            const body = { name, data, link_wrap }
            let result: Provider | undefined
            if (provider?.id) {
                const { data: updated } = await oapiClient.PATCH(
                    "/api/admin/projects/{projectID}/providers/{type}/{providerID}",
                    {
                        params: {
                            path: {
                                projectID: project.id,
                                type: module,
                                providerID: provider.id,
                            },
                        },
                        body,
                    },
                )
                result = updated
            } else {
                const { data: created } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/providers/{type}",
                    {
                        params: {
                            path: {
                                projectID: project.id,
                                type: module,
                            },
                        },
                        body,
                    },
                )
                result = created
            }
            if (result) onChange(result)
        } finally {
            setIsSaving(false)
        }
    }

    const dataSchema = extractDataSchema(meta)

    return (
        <form onSubmit={form.handleSubmit(handleSubmit)} className="grid gap-4">
            {provider?.id ? (
                provider?.setup &&
                provider.setup.length > 0 && (
                    <>
                        <h4 className="text-sm font-medium">{t("details", "Details")}</h4>
                        {provider.setup.map((item) => (
                            <div key={item.name} className="grid gap-2">
                                <Label className="text-muted-foreground">{item.name}</Label>
                                <Input value={item.value} disabled />
                            </div>
                        ))}
                        <Separator />
                    </>
                )
            ) : (
                <div className="rounded-lg border bg-muted/50 p-3">
                    <p className="text-sm font-medium">{meta.name}</p>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "integration_setup_hint",
                            "Fill out the fields below to setup this integration. For more information on this integration please see the documentation on our website.",
                        )}
                    </p>
                </div>
            )}

            <h4 className="text-sm font-medium">{t("config", "Config")}</h4>
            <div className="grid gap-2">
                <Label className="inline-flex items-center gap-1">
                    {t("name")} <span className="text-destructive">*</span>
                </Label>
                <Input {...form.register("name")} />
            </div>

            {dataSchema && <FormSchemaFields parent="data" schema={dataSchema} form={form} />}

            <DialogFooter className="pt-2">
                <Button type="submit" disabled={isSaving}>
                    {isSaving
                        ? t("saving", "Saving...")
                        : provider?.id
                          ? t("update_integration", "Update Integration")
                          : t("create_integration", "Create Integration")}
                </Button>
            </DialogFooter>
        </form>
    )
}

interface IntegrationModalProps {
    open: boolean
    onClose: (open: boolean) => void
    provider?: ProviderWithExtras
    onChange: (provider: Provider) => void
}

export default function IntegrationModal({
    open,
    onClose,
    onChange,
    provider,
}: IntegrationModalProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
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
    const [meta, setMeta] = useState<ProviderMeta | undefined>()

    const derivedMeta = useMemo(
        () => options?.find((item) => item.type === provider?.module),
        [options, provider],
    )

    const activeMeta = meta ?? derivedMeta

    const handleChange = (provider: Provider) => {
        onChange(provider)
        onClose(false)
        setMeta(undefined)
    }

    // External integration — simple info dialog
    if (provider?.external_id) {
        return (
            <Dialog open={open} onOpenChange={onClose}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("external_integration_title")}</DialogTitle>
                        <DialogDescription>{t("external_integration_alert")}</DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => onClose(false)}>
                            {t("close")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        )
    }

    const title = activeMeta
        ? provider?.id
            ? `${provider.name} (${activeMeta.name})`
            : t("setup_integration", "Setup Integration")
        : t("integrations")

    return (
        <Dialog
            open={open}
            onOpenChange={(isOpen) => {
                onClose(isOpen)
                if (!isOpen) setMeta(undefined)
            }}
        >
            <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{title}</DialogTitle>
                    {!activeMeta && (
                        <DialogDescription>
                            {t(
                                "pick_integration_hint",
                                "To get started, pick one of the integrations from the list below.",
                            )}
                        </DialogDescription>
                    )}
                </DialogHeader>

                {!activeMeta ? (
                    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                        {options?.map((option) => (
                            <button
                                key={option.type}
                                type="button"
                                className="flex flex-col items-center gap-2 rounded-lg border p-4 text-center transition-colors hover:bg-accent hover:text-accent-foreground cursor-pointer"
                                onClick={() => setMeta(option)}
                            >
                                {option.icon && (
                                    <img
                                        src={option.icon}
                                        alt={option.name}
                                        className="h-10 w-10 rounded-md object-contain"
                                    />
                                )}
                                <div>
                                    <p className="text-sm font-medium">{option.name}</p>
                                    <div className="flex gap-1 justify-center mt-1">
                                        {option.channels?.map((ch) => (
                                            <span
                                                key={ch}
                                                className="text-xs text-muted-foreground"
                                            >
                                                {snakeToTitle(ch)}
                                            </span>
                                        ))}
                                    </div>
                                </div>
                            </button>
                        ))}
                    </div>
                ) : (
                    <>
                        {!provider?.id && (
                            <Button
                                variant="ghost"
                                size="sm"
                                className="w-fit"
                                onClick={() => setMeta(undefined)}
                            >
                                <ChevronLeft className="mr-1 h-4 w-4" />
                                {t("integrations")}
                            </Button>
                        )}
                        <IntegrationForm
                            project={project}
                            provider={provider}
                            meta={activeMeta}
                            onChange={handleChange}
                        />
                    </>
                )}
            </DialogContent>
        </Dialog>
    )
}
