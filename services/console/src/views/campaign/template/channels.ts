import type { ComponentType } from "react";
import type { FieldValues, UseFormReturn } from "react-hook-form";
import type { Campaign, ChannelType, Template } from "@/types";
import { EmailContentPreview, EmailForm, EmailFormControl, EmailPreview } from "./mail/Setup";
import { TextForm, TextFormControl, TextPreview } from "./text/Setup";
import { PushForm, PushFormControl, PushPreview } from "./push/Setup";

export interface ChannelConfig<T extends FieldValues> {
    form: (campaign: Campaign, template?: Template) => UseFormReturn<T>;
    FormControl: ComponentType<{ campaign: Campaign; form: UseFormReturn<T>; disabled?: boolean }>;
    Preview: ComponentType<{ campaign: Campaign; form: UseFormReturn<T> }>;
    ContentPreview: ComponentType<{ campaign: Campaign; form: UseFormReturn<T>, edit?: boolean }>;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const channels: Partial<Record<ChannelType, ChannelConfig<any>>> = {
    email: {
        form: EmailForm,
        FormControl: EmailFormControl,
        Preview: EmailPreview,
        ContentPreview: EmailContentPreview,
    },
    text: {
        form: TextForm,
        FormControl: TextFormControl,
        Preview: TextPreview,
        ContentPreview: TextPreview,
    },
    push: {
        form: PushForm,
        FormControl: PushFormControl,
        Preview: PushPreview,
        ContentPreview: PushPreview,
    },
};
