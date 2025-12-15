import { Controller, useForm } from "react-hook-form";
import { useCallback, useContext, useEffect } from "react";
import { CampaignContext, ProjectContext } from "@/contexts";
import { useTranslation } from "react-i18next";
import api from "@/api";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";

import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ProviderSelect } from "@/components/provider/select";
import { CampaignWorkflowContext } from "./contexts";

const schema = z.object({
    name: z.string().min(1, "Name is required"),
    provider_id: z.string().min(1, "Provider is required"),
});

type FormData = z.infer<typeof schema>;

export default function CampaignSetup() {
    const [campaign, setCampaign] = useContext(CampaignContext);
    const [project] = useContext(ProjectContext);
    const { onSubmit: onNext } = useContext(CampaignWorkflowContext);
    const { t } = useTranslation();

    const form = useForm<FormData>({
        resolver: zodResolver(schema),
        defaultValues: {
            name: campaign.name || "",
            provider_id: campaign.provider_id,
        },
    });

    onNext(async () => {
        const isValid = await form.trigger();
        if (!isValid) {
            return false;
        }

        const data = form.getValues();
        const updated = await api.campaigns.update(project.id, campaign.id, {
            name: data.name,
            provider_id: data.provider_id,
        });

        setCampaign(updated);

        if (campaign.templates?.length === 0) {
            const template = await api.templates.create(project.id, {
                campaign_id: campaign.id,
                locale: project.locale,
                type: campaign.channel,
                data: {}
            });

            setCampaign({
                ...campaign,
                templates: [template]
            });
        }

        return true;
    });

    return (
        <div className="flex flex-1 items-center justify-center bg-muted/20">
            <div className="w-full max-w-2xl space-y-6 bg-background p-8 rounded-lg border">
                <div className="space-y-2">
                    <h1 className="text-2xl font-semibold">{t('campaign.setup.title')}</h1>
                    <p className="text-muted-foreground">{t('campaign.setup.description')}</p>
                </div>

                <form className="space-y-6">
                    <FieldGroup>
                        <Controller
                            name="name"
                            control={form.control}
                            render={({ field, fieldState }) => (
                                <Field data-invalid={fieldState.invalid} className="gap-2">
                                    <FieldLabel htmlFor="campaign-name">{t('campaign.setup.form.name.label')}</FieldLabel>
                                    <Input
                                        {...field}
                                        id="campaign-name"
                                        aria-invalid={fieldState.invalid}
                                        placeholder=""
                                        autoComplete="off"
                                    />
                                    <FieldDescription>
                                        {t('campaign.setup.form.name.description')}
                                    </FieldDescription>
                                    {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
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
                                    <FieldLabel htmlFor="campaign-provider">{t('campaign.setup.form.provider.label')}</FieldLabel>
                                    <ProviderSelect
                                        value={field.value}
                                        onChange={field.onChange}
                                        channel={campaign.channel}
                                    />
                                    <FieldDescription className="whitespace-pre-line">
                                        {t('campaign.setup.form.provider.description')}
                                    </FieldDescription>
                                    {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                                </Field>
                            )}
                        />
                    </FieldGroup>
                </form>
            </div>
        </div>
    );
}
