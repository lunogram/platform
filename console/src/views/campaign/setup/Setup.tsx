import { Controller, useForm } from "react-hook-form"
import { useCallback, useContext, useMemo, useState } from "react"
import { CampaignContext, ProjectContext } from "@/contexts"
import { useTranslation } from "react-i18next"
import api from "@/api"
import { useResolver } from "@/hooks"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Subscription } from "@/types"

import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { ProviderSelect } from "@/components/provider/select"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { useNavigate } from "react-router"

const schema = z.object({
    name: z.string().min(1, "Name is required"),
    provider_id: z.string("Provider is required"),
})

type FormData = z.infer<typeof schema>

export default function CampaignSetup() {
    const [campaign, setCampaign] = useContext(CampaignContext)
    const [isSubmitting, setIsSubmitting] = useState(false)
    const [project] = useContext(ProjectContext)
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [isTransactional, setTransactional] = useState(campaign.transactional ?? false)
    const [subscriptionId, setSubscriptionId] = useState<string>(campaign.subscription_id ?? "")

    const [subscriptions] = useResolver(
        useCallback(async (): Promise<Subscription[]> => {
            if (!project?.id) return []
            const result = await api.subscriptions.search(project.id, { limit: 100 })
            return result.results ?? []
        }, [project?.id]),
    )

    const filteredSubscriptions = useMemo(() => {
        return (subscriptions ?? []).filter((s) => {
            const ch = (s.channel) === "sms" ? "text" : s.channel
            return ch === campaign.channel
        })
    }, [subscriptions, campaign.channel])

    const form = useForm<FormData>({
        resolver: zodResolver(schema),
        defaultValues: {
            name: campaign.name || "",
            provider_id: campaign.provider_id,
        },
    })

    const onSubmit = async (data: FormData) => {
        setIsSubmitting(true)
        try {
            const updated = await api.campaigns.update(project.id, campaign.id, {
                name: data.name,
                provider_id: data.provider_id,
                transactional: isTransactional,
                subscription_id: isTransactional ? undefined : subscriptionId,
            })

            setCampaign(updated)

            if (campaign.templates?.length === 0) {
                const template = await api.campaigns.templates.create(project.id, campaign.id, {
                    locale: project.locale,
                    data: {},
                })

                setCampaign({
                    ...campaign,
                    templates: [template],
                })
            }

            const template =
                campaign.templates.find((template) => template.locale === project.locale) ??
                campaign.templates[0]
            await navigate(
                `/projects/${project.id}/campaigns/${campaign.id.toString()}/templates/${template.id.toString()}`,
            )
        } finally {
            setIsSubmitting(false)
        }
    }

    return (
        <div className="flex h-screen items-center justify-center bg-muted/20">
            <div className="w-full max-w-2xl space-y-6 bg-background p-8 rounded-lg border">
                <div className="space-y-2">
                    <h1 className="text-2xl font-semibold">{t("campaign.setup.title")}</h1>
                    <p className="text-muted-foreground">{t("campaign.setup.description")}</p>
                </div>

                <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                    <FieldGroup>
                        <Controller
                            name="name"
                            control={form.control}
                            render={({ field, fieldState }) => (
                                <Field data-invalid={fieldState.invalid} className="gap-2">
                                    <FieldLabel htmlFor="campaign-name">
                                        {t("campaign.setup.form.name.label")}
                                    </FieldLabel>
                                    <Input
                                        {...field}
                                        id="campaign-name"
                                        aria-invalid={fieldState.invalid}
                                        placeholder=""
                                        autoComplete="off"
                                    />
                                    <FieldDescription>
                                        {t("campaign.setup.form.name.description")}
                                    </FieldDescription>
                                    {fieldState.invalid && (
                                        <FieldError errors={[fieldState.error]} />
                                    )}
                                </Field>
                            )}
                        />
                    </FieldGroup>

                    <FieldGroup>
                        <Controller
                            name="provider_id"
                            control={form.control}
                            render={({ field, fieldState }) => (
                                <Field data-invalid={fieldState.invalid} className="gap-2">
                                    <FieldLabel htmlFor="campaign-provider">
                                        {t("campaign.setup.form.provider.label")}
                                    </FieldLabel>
                                    <ProviderSelect
                                        value={field.value}
                                        onChange={field.onChange}
                                        channel={campaign.channel}
                                    />
                                    <FieldDescription className="whitespace-pre-line">
                                        {t("campaign.setup.form.provider.description")}
                                    </FieldDescription>
                                    {fieldState.invalid && (
                                        <FieldError errors={[fieldState.error]} />
                                    )}
                                </Field>
                            )}
                        />
                    </FieldGroup>

                    <div className="flex items-center justify-between rounded-md border p-3">
                        <div className="space-y-1">
                            <Label htmlFor="transactional-toggle">
                                {t("campaign.transactional", "Transactional")}
                            </Label>
                            <p className="text-sm text-muted-foreground">
                                {t(
                                    "campaign.transactional.help",
                                    "When enabled, subscription preference is ignored.",
                                )}
                            </p>
                        </div>
                        <Switch
                            id="transactional-toggle"
                            checked={isTransactional}
                            onCheckedChange={(checked) => {
                                setTransactional(checked)
                                if (checked) setSubscriptionId("")
                            }}
                        />
                    </div>

                    {!isTransactional && (
                        <FieldGroup>
                            <Field className="gap-2">
                                <FieldLabel htmlFor="subscription-select">
                                    {t("campaign.subscription", "Subscription")}
                                </FieldLabel>
                                <Select value={subscriptionId} onValueChange={setSubscriptionId}>
                                    <SelectTrigger id="subscription-select">
                                        <SelectValue
                                            placeholder={t(
                                                "campaign.subscription.placeholder",
                                                "Select subscription",
                                            )}
                                        />
                                    </SelectTrigger>
                                    <SelectContent className="z-[1100]">
                                        {filteredSubscriptions.length === 0 && (
                                            <SelectItem value="__empty" disabled>
                                                {t(
                                                    "campaign.subscription.empty",
                                                    "No subscriptions for this channel",
                                                )}
                                            </SelectItem>
                                        )}
                                        {filteredSubscriptions.map((subscription) => (
                                            <SelectItem
                                                key={subscription.id}
                                                value={subscription.id}
                                            >
                                                {subscription.name}
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </Field>
                        </FieldGroup>
                    )}

                    <div className="flex justify-end">
                        <Button type="submit" isLoading={isSubmitting} disabled={isSubmitting}>
                            {t("actions.submit")}
                        </Button>
                    </div>
                </form>
            </div>
        </div>
    )
}
