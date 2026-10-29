import type { ComponentType } from "react"
import type { FieldValues, UseFormReturn } from "react-hook-form"
import type { Campaign, ChannelType, Template } from "@/types"
import { EmailContentPreview, EmailFormControl, EmailPreview } from "./mail/Setup"
import { useEmailForm } from "./mail/useEmailForm"
import { TextFormControl, TextPreview } from "./text/Setup"
import { useTextForm } from "./text/useTextForm"
import { PushFormControl, PushPreview } from "./push/Setup"
import { usePushForm } from "./push/usePushForm"
import { InboxFormControl, InboxPreview } from "./inbox/Setup"
import { useInboxForm } from "./inbox/useInboxForm"
export interface ChannelConfig<T extends FieldValues> {
    /**
     * A hook. Call it unconditionally at the top of a component, after taking it
     * out of the config (`const { useForm: useChannelForm } = config`), so the
     * rules-of-hooks lint can check the call.
     */
    useForm: (campaign: Campaign, template?: Template) => UseFormReturn<T>
    FormControl: ComponentType<{ campaign: Campaign; form: UseFormReturn<T>; disabled?: boolean }>
    Preview: ComponentType<{ campaign: Campaign; form: UseFormReturn<T> }>
    ContentPreview: ComponentType<{ campaign: Campaign; form: UseFormReturn<T>; edit?: boolean }>
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const channels: Partial<Record<ChannelType, ChannelConfig<any>>> = {
    email: {
        useForm: useEmailForm,
        FormControl: EmailFormControl,
        Preview: EmailPreview,
        ContentPreview: EmailContentPreview,
    },
    sms: {
        useForm: useTextForm,
        FormControl: TextFormControl,
        Preview: TextPreview,
        ContentPreview: TextPreview,
    },
    push: {
        useForm: usePushForm,
        FormControl: PushFormControl,
        Preview: PushPreview,
        ContentPreview: PushPreview,
    },
    inbox: {
        useForm: useInboxForm,
        FormControl: InboxFormControl,
        Preview: InboxPreview,
        ContentPreview: InboxPreview,
    },
}
