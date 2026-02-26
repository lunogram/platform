import { Controller, useForm } from "react-hook-form";
import { useContext, useState } from "react";
import { CampaignContext, ProjectContext } from "@/contexts";
import { useTranslation } from "react-i18next";
import { oapiClient } from "@/oapi/client";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";

import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ProviderSelect } from "@/components/provider/select";
import { Button } from "@/components/ui/button";
import { useNavigate } from "react-router";

const schema = z.object({
    name: z.string().min(1, "Name is required"),
    provider_id: z.string("Provider is required"),
});

type FormData = z.infer<typeof schema>;

export default function CampaignSetup() {
    const [campaign, setCampaign] = useContext(CampaignContext);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [project] = useContext(ProjectContext);
    const { t } = useTranslation();
    const navigate = useNavigate();

    const form = useForm<FormData>({
        resolver: zodResolver(schema),
        defaultValues: {
            name: campaign.name || "",
            provider_id: campaign.provider_id,
        },
    });

    const onSubmit = async (data: FormData) => {
        setIsSubmitting(true);
        try {
            const updated = await oapiClient.PATCH("/api/admin/projects/{projectID}/campaigns/{campaignID}", {
                params: {
                    path: {
                        projectID: project.id,
                        campaignID: campaign.id,
                    }
                },
                body: {
                    name: data.name,
                    provider_id: data.provider_id,
                }
            });

            if (!updated.data) {
                return;
            }

            setCampaign(updated.data);

            

            if (updated.data.templates?.length === 0) {
                const template = await oapiClient.POST("/api/admin/projects/{projectID}/campaigns/{campaignID}/templates", {
                    params: {
                        path: {
                            projectID: project.id,
                            campaignID: campaign.id,
                        }
                    },
                    body: {
                        name: "Default Template",
                        locale: project.locale,
                    }
                });
                if (template.data?.id) {
                    navigate(`/projects/${project.id}/campaigns/${campaign.id.toString()}/templates/${template.data.id.toString()}`);
                }
                return;
            }

            const template = updated.data.templates.find((template) => template.locale === project.locale) ?? updated.data.templates[0];
            navigate(`/projects/${project.id}/campaigns/${campaign.id.toString()}/templates/${template.id.toString()}`);
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <div className="flex h-screen items-center justify-center bg-muted/20">
            <div className="w-full max-w-2xl space-y-6 bg-background p-8 rounded-lg border">
                <div className="space-y-2">
                    <h1 className="text-2xl font-semibold">{t('campaign.setup.title')}</h1>
                    <p className="text-muted-foreground">{t('campaign.setup.description')}</p>
                </div>

                <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
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

                    <div className="flex justify-end">
                        <Button
                            type="submit"
                            isLoading={isSubmitting}
                            disabled={isSubmitting}
                        >
                            {t('actions.submit')}
                        </Button>
                    </div>
                </form>
            </div>
        </div>
    );
}
