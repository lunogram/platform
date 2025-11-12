import { Controller, useForm, type UseFormReturn } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { Campaign, Template, User } from "@/types";
import { Bell } from 'lucide-react';
import { useTranslation } from "react-i18next";
import { useContext, useState } from "react";
import { ProjectContext } from "@/contexts";
import * as z from "zod";

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { UserSelection } from "../UserSelection";

const pushSetupFormSchema = z.object({
    title: z.string("Title is required").min(1, "Title is required"),
    body: z.string("Body is required").min(1, "Body is required"),
    custom: z.record(z.string(), z.unknown()).optional(),
});

export function PushForm(_campaign: Campaign, template?: Template) {
    const formSchema = pushSetupFormSchema.extend({});

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues: {
            title: template?.data.title,
            body: template?.data.body,
            custom: template?.data.custom,
        },
    });
}

interface PushFormControlProps {
    campaign: Campaign;
    form: UseFormReturn<z.infer<typeof pushSetupFormSchema>>;
    disabled?: boolean;
}

export function PushFormControl({ form, disabled = false }: PushFormControlProps) {
    const { t } = useTranslation();

    return (
        <FieldGroup className="mt-7">
            <Controller
                name="title"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-title">{t('campaign.setup.channels.push.title.label')}</FieldLabel>
                        <Input
                            {...field}
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                            disabled={disabled}
                            readOnly={disabled}
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            <Controller
                name="body"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-message">{t('campaign.setup.channels.push.body.label')}</FieldLabel>
                        <Textarea
                            {...field}
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                            disabled={disabled}
                            readOnly={disabled}
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            {/* TODO: expose custom field as JSON */}
        </FieldGroup>
    )
}

const randomNotifications = [
    ["Welcome to our App!", "Thank you for installing our app. We're excited to have you on board."],
    ["Don't miss out!", "Check out the latest features we've added to enhance your experience."],
    ["Special Offer!", "Get 20% off on your next purchase. Limited time offer!"],
    ["Update Available", "A new version of the app is available. Update now for the best experience."],
    ["Weekly Summary", "Here's what you've missed this week. Stay updated with the latest news."],
];

function randomNotification() {
    const index = Math.floor(Math.random() * randomNotifications.length);
    return randomNotifications[index];
}

export interface PushSetupProps {
    campaign: Campaign;
    form: UseFormReturn<z.infer<typeof pushSetupFormSchema>>;
}

export function PushPreview({ campaign: _campaign, form }: PushSetupProps) {
    const [project] = useContext(ProjectContext);
    const [selectedUser, setSelectedUser] = useState<User | null>(null);

    const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

    const [[placeholderTitle, placeholderBody]] = useState(() => randomNotification());
    const title = form.watch('title') ?? placeholderTitle;
    const body = form.watch('body') ?? placeholderBody;

    return (
        <>
            <div className="mb-4">
                <UserSelection
                    projectId={project?.id}
                    value={selectedUser}
                    onChange={setSelectedUser}
                />
            </div>
            <div className="flex w-full items-end justify-end">
                <div className="w-full max-w-md">
                    <div className="bg-white rounded-2xl shadow-xl overflow-hidden border border-gray-200">
                        <div className="px-4 py-3 flex items-start gap-3">
                            <div className="flex-shrink-0 w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                <Bell className="w-5 h-5 text-white" />
                            </div>

                            <div className="flex-1 flex gap-1 flex-col">
                                <div className="flex items-center justify-between">
                                    <span className="text-sm font-semibold text-gray-900">{title}</span>
                                    <span className="text-xs text-gray-500">{time}</span>
                                </div>

                                <p className="text-sm text-gray-600 line-clamp-3">
                                    {body}
                                </p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </>
    );
}
