import { Controller, useForm, type UseFormReturn } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { Campaign, Provider, Template } from "@/types";
import { SetupBaseFormSchema } from "../schemas";
import { cn } from "@/utils";
import * as z from "zod";

import { Input } from "@/components/ui/input";

import {
    Field,
    FieldDescription,
    FieldError,
    FieldGroup,
    FieldLabel,
} from "@/components/ui/field";

const emailSetupFormSchema = SetupBaseFormSchema.extend({
    subject: z.string().min(1, "Subject is required"),
    fromName: z.string().optional(),
    fromEmail: z.email("Invalid from email address").optional(),
    replyTo: z.email("Invalid reply-to email address").optional(),
});


const randomSubjects = [
    "You won't believe what we have for you...",
    "Your exclusive invitation inside 🎉",
    "Last chance: Don't miss out!",
    "We thought you'd love this",
    "Something special just for you",
    "Your weekly update is here",
    "Quick question for you...",
    "This might interest you",
    "A gift from us to you 🎁",
    "Breaking news you need to see"
];

function randomSubject() {
    const index = Math.floor(Math.random() * randomSubjects.length);
    return randomSubjects[index];
}

export function EmailForm(campaign: Campaign, template: Template) {
    const formSchema = emailSetupFormSchema.extend({
        fromEmail: campaign?.provider?.data.default_from
            ? z.string().optional()
            : z.email("Invalid from email address"),
        fromName: campaign?.provider?.data.default_from_name
            ? z.string().optional()
            : z.string().min(1, "From name is required"),
    });

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues: {
            name: campaign.name,
            provider_id: campaign.provider_id ?? '',
            subject: template.data.subject ?? randomSubject(),
            fromName: template.data.fromName,
            fromEmail: template.data.fromEmail,
            replyTo: template.data.replyTo,
        },
    });
}

interface EmailFormControlProps {
    campaign: Campaign;
    form: UseFormReturn<z.infer<typeof emailSetupFormSchema>>;
}

export function EmailFormControl({ campaign, form }: EmailFormControlProps) {
    return (
        <FieldGroup className="mt-7">
            <Controller
                name="subject"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-subject">Subject</FieldLabel>
                        <Input
                            {...field}
                            id="form-rhf-demo-subject"
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            <Controller
                name="fromName"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor="form-rhf-demo-fromName">From Name</FieldLabel>
                        <Input
                            {...field}
                            id="form-rhf-demo-fromName"
                            aria-invalid={fieldState.invalid}
                            placeholder={campaign?.provider?.data.default_from_name || ''}
                            autoComplete="off"
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            <Controller
                name="fromEmail"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor="form-rhf-demo-fromEmail">
                            From Email
                        </FieldLabel>
                        <Input
                            {...field}
                            id="form-rhf-demo-fromEmail"
                            aria-invalid={fieldState.invalid}
                            placeholder={campaign?.provider?.data.default_from || ''}
                            disabled={campaign?.provider?.data.default_from_locked}
                            autoComplete="off"
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            <Controller
                name="replyTo"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor="form-rhf-demo-replyTo">
                            Reply-To Email
                        </FieldLabel>
                        <Input
                            {...field}
                            id="form-rhf-demo-replyTo"
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
        </FieldGroup>
    );
}

export interface EmailSetupProps {
    campaign: Campaign;
    form: UseFormReturn<z.infer<typeof emailSetupFormSchema>>;
}

export function EmailPreview({ campaign, form }: EmailSetupProps) {
    const loadingSubjects = 3;
    const { subject, fromName, fromEmail } = form.watch();

    const displayFromName = fromName || campaign?.provider?.data.default_from_name || '';
    const displayFromEmail = fromEmail || campaign?.provider?.data.default_from || '';

    return (
        <>
            <div
                className="bg-white border rounded-md shadow-sm w-full overflow-hidden"
            >
                <div className="flex items-center gap-3 px-4 py-2 border-b">
                    <div className="flex items-center gap-3">
                        <input
                            type="checkbox"
                            className="h-4 w-4 rounded border-gray-300"
                        />
                        <div className="flex items-center gap-2 text-gray-400">
                            <StarIcon />
                            <ChevronIcon />
                        </div>
                    </div>
                    <div className="w-2/5 flex items-center gap-1 truncate min-w-0">
                        {displayFromName && (
                            <span className="font-semibold text-gray-900 whitespace-nowrap">
                                {displayFromName}
                            </span>
                        )}
                        {displayFromEmail && (
                            <span className="text-gray-400 whitespace-nowrap truncate">
                                &lt;{displayFromEmail}&gt;
                            </span>
                        )}
                    </div>
                    <div className="w-3/5 font-medium text-sm truncate">{subject}</div>
                </div>

                <div>
                    {Array.from({ length: loadingSubjects }).map((_, index) => (
                        <div key={index} className="flex items-center gap-3 px-4 py-2">
                            <div className="flex items-center gap-3">
                                <input
                                    type="checkbox"
                                    className="h-4 w-4 rounded border-gray-300"
                                    disabled
                                />
                                <div className="flex items-center gap-2 text-gray-400">
                                    <StarIcon />
                                    <ChevronIcon />
                                </div>
                            </div>
                            <div className="w-2/5">
                                <div className="h-3 w-1/2 bg-gray-200 rounded"></div>
                            </div>
                            <div className="w-3/5 flex gap-3">
                                <div className="h-3 w-1/5 bg-gray-200 rounded"></div>
                                <div className="h-3 w-4/5 bg-gray-200 rounded"></div>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
            <p className="text-center leading-7 mt-4 text-muted-foreground">
                Preview only, email clients may display differently.
            </p>
        </>
    );
}

function StarIcon() {
    return (
        <svg
            className="h-4 w-4 opacity-40"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M12 17.75L18.25 21l-1.5-7 5.25-4.75-7-.75L12 2 9 8.5l-7 .75L7.25 14l-1.5 7L12 17.75z" />
        </svg>
    )
}

function ChevronIcon() {
    return (
        <svg
            className="h-4 w-4 opacity-40"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M9 5l7 7-7 7" />
        </svg>
    )
}
