import { useCallback, useContext, useEffect, useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { ChevronLeft } from "lucide-react"
import api from "../../api"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle } from "../../utils"
import type { Project, Provider, ProviderCreateParams, ProviderMeta } from "../../types"
import type { SchemaProperty } from "@/components/schema-fields"

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
interface IntegrationFormParams {
    project: Project
    meta: ProviderMeta
    provider?: Provider
    onChange: (provider: Provider) => void
}

export function IntegrationForm({
    project,
    provider: defaultProvider,
    onChange,
    meta,
}: IntegrationFormParams) {
    const { t } = useTranslation()
    const [provider, setProvider] = useState<Provider | undefined>(defaultProvider)
    const [isSaving, setIsSaving] = useState(false)

    const module = meta.type
    const channel = meta.group

    useEffect(() => {
        if (defaultProvider) {
            api.providers
                .get(
                    project.id,
                    defaultProvider.channel,
                    defaultProvider.module,
                    defaultProvider.id,
                )
                .then((provider) => setProvider(provider))
                .catch(() => {})
        }
    }, [project.id, defaultProvider])

    const form = useForm<ProviderCreateParams>({
        values: provider
            ? {
                  name: provider.name,
                  data: provider.data,
                  module,
                  channel,
              }
            : { name: "", data: {}, module, channel },
    })

    const handleSubmit = async (values: ProviderCreateParams) => {
        setIsSaving(true)
        try {
            const params = { ...values, module, channel }
            const result = provider?.id
                ? await api.providers.update(project.id, provider.id, params)
                : await api.providers.create(project.id, params)
            onChange(result)
        } finally {
            setIsSaving(false)
        }
    }

    return (
        <form onSubmit={form.handleSubmit(handleSubmit)} className="grid gap-4">
            {provider?.id ? (
                provider?.setup?.length > 0 && (
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
                <Input {...form.register("name", { required: true })} />
            </div>

            <FormSchemaFields
                parent="data"
                schema={
                    Array.isArray(meta.schema.properties)
                        ? meta.schema.properties.find((p: SchemaProperty) => p.name === "data")
                              ?.schema
                        : meta.schema.properties?.data
                }
                form={form}
            />

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
    provider?: Provider
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
        useCallback(async () => await api.providers.options(project.id), [project]),
    )
    const [meta, setMeta] = useState<ProviderMeta | undefined>()

    const derivedMeta = useMemo(
        () =>
            options?.find(
                (item) => item.group === provider?.channel && item.type === provider?.module,
            ),
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
                                key={`${option.group}${option.type}`}
                                type="button"
                                className="flex flex-col items-center gap-2 rounded-lg border p-4 text-center transition-colors hover:bg-accent hover:text-accent-foreground cursor-pointer"
                                onClick={() => setMeta(option)}
                            >
                                {option.icon && (
                                    <img
                                        src={option.icon}
                                        alt={option.name}
                                        className="h-10 w-10 rounded-md"
                                    />
                                )}
                                <div>
                                    <p className="text-sm font-medium">{option.name}</p>
                                    <p className="text-xs text-muted-foreground">
                                        {snakeToTitle(option.group)}
                                    </p>
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
