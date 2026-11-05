import type { ComponentType } from "react"
import type { FieldValues, UseFormReturn } from "react-hook-form"
import type { Campaign, ChannelType, Template } from "@/types"
import type { TemplateChanges } from "@/validation/campaign/template/data"
import type { EmailFormValues } from "@/validation/campaign/template/mail/setup"
import type { TextFormValues } from "@/validation/campaign/template/text/setup"
import type { PushFormValues } from "@/validation/campaign/template/push/setup"
import type { InboxFormValues } from "@/validation/campaign/template/inbox/setup"
import { EmailFormControl, EmailPreview } from "./mail/Setup"
import { EmailContentPreview } from "./mail/ContentPreview"
import { emailTemplateChanges, useEmailForm } from "./mail/useEmailForm"
import { TextFormControl, TextPreview } from "./text/Setup"
import { textTemplateChanges, useTextForm } from "./text/useTextForm"
import { PushFormControl, PushPreview } from "./push/Setup"
import { pushTemplateChanges, usePushForm } from "./push/usePushForm"
import { InboxFormControl, InboxPreview } from "./inbox/Setup"
import { inboxTemplateChanges, useInboxForm } from "./inbox/useInboxForm"

export interface ChannelConfig<T extends FieldValues> {
    /**
     * A hook. Call it unconditionally at the top of a component, after taking it
     * out of the config (`const { useForm: useChannelForm } = config`), so the
     * rules-of-hooks lint can check the call.
     */
    useForm: (campaign: Campaign, template?: Template) => UseFormReturn<T>
    /** What the form changes on its template when the Content step saves. */
    toTemplate: (values: T) => TemplateChanges
    FormControl: ComponentType<{ campaign: Campaign; form: UseFormReturn<T>; disabled?: boolean }>
    Preview: ComponentType<{ campaign: Campaign; form: UseFormReturn<T> }>
    ContentPreview: ComponentType<{ campaign: Campaign; form: UseFormReturn<T>; edit?: boolean }>
}

interface ChannelForms {
    email: EmailFormValues
    sms: TextFormValues
    push: PushFormValues
    inbox: InboxFormValues
}

const channels: { [C in ChannelType]: ChannelConfig<ChannelForms[C]> } = {
    email: {
        useForm: useEmailForm,
        toTemplate: emailTemplateChanges,
        FormControl: EmailFormControl,
        Preview: EmailPreview,
        ContentPreview: EmailContentPreview,
    },
    sms: {
        useForm: useTextForm,
        toTemplate: textTemplateChanges,
        FormControl: TextFormControl,
        Preview: TextPreview,
        ContentPreview: TextPreview,
    },
    push: {
        useForm: usePushForm,
        toTemplate: pushTemplateChanges,
        FormControl: PushFormControl,
        Preview: PushPreview,
        ContentPreview: PushPreview,
    },
    inbox: {
        useForm: useInboxForm,
        toTemplate: inboxTemplateChanges,
        FormControl: InboxFormControl,
        Preview: InboxPreview,
        ContentPreview: InboxPreview,
    },
}

/**
 * Calls render with a channel's config. Indexing `channels[channel]` would give
 * a union of configs whose form types can't be matched up; the generic callback
 * keeps one config's form type intact instead.
 *
 * @returns undefined for a channel the console does not support.
 */
export function withChannel<R>(
    channel: ChannelType,
    render: <T extends FieldValues>(config: ChannelConfig<T>) => R,
): R | undefined {
    switch (channel) {
        case "email":
            return render(channels.email)
        case "sms":
            return render(channels.sms)
        case "push":
            return render(channels.push)
        case "inbox":
            return render(channels.inbox)
        default:
            return undefined
    }
}
