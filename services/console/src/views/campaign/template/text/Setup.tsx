import { Controller, useForm, type UseFormReturn } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Ellipsis, UserRound } from 'lucide-react';
import type { Campaign, Template, User } from "@/types";
import { useTranslation } from "react-i18next";
import * as z from "zod";

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { UserSelection } from "../UserSelection";
import { useContext, useState } from "react";
import { ProjectContext } from "@/contexts";

const textSetupFormSchema = z.object({
    message: z.string("Message is required").min(1, "Message is required"),
});

export function TextForm(_campaign: Campaign, template?: Template) {
    const formSchema = textSetupFormSchema.extend({});

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues: {
            message: template?.data.message,
        },
    });
}

interface TextFormControlProps {
    campaign: Campaign;
    form: UseFormReturn<z.infer<typeof textSetupFormSchema>>;
    disabled?: boolean;
}

export function TextFormControl({ form, disabled = false }: TextFormControlProps) {
    const { t } = useTranslation();

    return (
        <FieldGroup className="mt-7">
            <Controller
                name="message"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-message">{t('campaign.setup.channels.text.message.label')}</FieldLabel>
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
        </FieldGroup>
    )
}

export interface TextSetupProps {
    campaign: Campaign;
    form: UseFormReturn<z.infer<typeof textSetupFormSchema>>;
}

export function TextPreview({ campaign: _campaign, form }: TextSetupProps) {
    const [project] = useContext(ProjectContext);
    const { t } = useTranslation();
    const [selectedUser, setSelectedUser] = useState<User | null>(null);

    const message = form.watch('message');
    const phoneNumber = project.name.charAt(0).toUpperCase() + project.name.slice(1);

    return (
        <div className="flex h-full items-center flex-col">
            <div className="mb-8 m-auto">
                <UserSelection
                    projectId={project?.id}
                    value={selectedUser}
                    onChange={setSelectedUser}
                />
            </div>
            <div className="w-[390px] h-[533px] bg-zinc-900 rounded-t-[70px] p-3 pb-0 shadow-2xl">
                <div className="w-full h-full bg-white rounded-t-[58px] overflow-hidden flex flex-col">
                    <div className="h-12 bg-white flex items-start justify-center px-8 pt-3">
                        <div className="w-32 h-8 bg-zinc-900 rounded-full" />
                    </div>

                    <div className="bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-center">
                        <div className="flex flex-col items-center">
                            <div className="w-12 h-12 bg-gray-300 rounded-full flex items-center justify-center mb-1">
                                <UserRound className="w-7 h-7 text-gray-500" strokeWidth={1.5} />
                            </div>
                            <span className="text-sm font-medium">{phoneNumber}</span>
                        </div>
                    </div>

                    <div className="flex-1 bg-white px-4 py-6 overflow-y-auto">
                        <div className="flex flex-col items-center mb-6">
                            <span className="text-gray-500 text-xs">{t('campaign.setup.channels.text.text_message_label')}</span>
                            <span className="text-gray-400 text-xs">{t('campaign.setup.channels.text.today')}</span>
                        </div>

                        <div className="flex justify-start mb-6">
                            <div className="max-w-[75%]">
                                <div className="bg-gray-200 rounded-3xl rounded-bl-sm px-4 py-3">
                                    {message || <Ellipsis className="text-gray-500" />}
                                </div>
                            </div>
                        </div>

                        <div className="text-center">
                            <p className="text-gray-400 text-sm">{t('campaign.setup.channels.text.preview_disclaimer')}</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
