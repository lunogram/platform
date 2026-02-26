import { useContext, useState, useEffect } from "react";
import { CampaignContext, ProjectContext, TemplateContext as CurrentTemplateContext } from "@/contexts";
import type { User } from "@/types";
import { useTranslation } from "react-i18next";
import { oapiClient } from "@/oapi/client";

import { channels } from "./channels";

import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { TemplateWorkflowContext } from "./contexts";

export default function TemplateReview() {
    const [campaign] = useContext(CampaignContext);
    const [project] = useContext(ProjectContext);
    const { onSubmit } = useContext(TemplateWorkflowContext);
    const [template] = useContext(CurrentTemplateContext);
    const { t } = useTranslation();
    const [selectedUser, setSelectedUser] = useState<User | null>(null);

    useEffect(() => {
        if (!selectedUser && project?.id) {
            oapiClient.GET("/api/admin/projects/{projectID}/users", {
                params: {
                    path: {
                        projectID: project.id,
                    },
                    query: {
                        limit: 1,
                    },
                },            
            }).then((res) => {
            const users = res.data?.results || [];
            if (users.length > 0) {
                setSelectedUser(users[0]);
            }
        });
        }
    }, [project?.id, selectedUser]);

    if (!campaign || !project || !template) {
        return null;
    }

    const config = channels[campaign.channel];

    if (!config) {
        return null;
    }

    const form = config.form(campaign, template);
    const ChannelFormControl = config.FormControl;
    const ChannelPreview = config.ContentPreview;

    onSubmit(async () => {
        if (!template) {
            return false;
        }

        await oapiClient.PATCH("/api/admin/projects/{projectID}/campaigns/{campaignID}/templates/{templateID}", {
            params: {
                path: {
                    projectID: project.id,
                    campaignID: campaign.id,
                    templateID: template.id,
                }
            },
            body: {
                data: form.getValues(),
            }
        });

        return true;
    });

    return (
        <div className="flex flex-1 bg-muted/20 overflow-hidden">
            <div className="h-full overflow-y-auto w-2/5 bg-background p-8">
                <div className="mb-6">
                    <h1 className="text-2xl font-semibold">{t('campaign.review.title', 'Review')}</h1>
                    <p className="text-muted-foreground">{t('campaign.review.description', 'Review your campaign before sending.')}</p>
                </div>

                <div className="space-y-6">
                    <FieldGroup>
                        <Field className="gap-2">
                            <FieldLabel>{t('campaign.setup.form.name.label')}</FieldLabel>
                            <Input
                                value={campaign.name}
                                readOnly
                                disabled
                            />
                        </Field>

                        {campaign.provider && (
                            <Field className="gap-2">
                                <FieldLabel>{t('campaign.setup.form.provider.label')}</FieldLabel>
                                <Input
                                    value={campaign.provider.name}
                                    readOnly
                                    disabled
                                />
                            </Field>
                        )}
                    </FieldGroup>

                    <ChannelFormControl campaign={campaign} form={form} disabled />
                </div>
            </div>

            <div className="w-3/5 bg-background p-8 pb-0 border-l">
                <ChannelPreview campaign={campaign} form={form} />
            </div>
        </div>
    );
}
