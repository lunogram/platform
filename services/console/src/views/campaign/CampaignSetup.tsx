import { Controller, useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useContext, useState } from "react"
import * as z from "zod"
import { cn } from "@/utils";

import { ProjectContext } from "@/contexts"

import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"

import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"

import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "@/components/ui/popover"

import {
    Field,
    FieldDescription,
    FieldError,
    FieldGroup,
    FieldLabel,
} from "@/components/ui/field"
import { Check, ChevronsUpDown } from "lucide-react";

const setupBaseFormSchema = z.object({
    name: z.string().min(1, "Campaign name is required"),
})

const setupFormEmail = setupBaseFormSchema.extend(z.object({
    subject: z.string().min(1, "Subject is required"),
    fromName: z.string().min(1, "From name is required"),
    fromEmail: z.email("Invalid from email address"),
    replyTo: z.email("Invalid reply-to email address").optional(),
}).shape)


interface EmailFormControlProps {
    form: UseFormReturn<z.infer<typeof setupFormEmail>>
}

function EmailFormControl({ form }: EmailFormControlProps) {
    return (
        <FieldGroup>

            <Controller
                name="name"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-name">
                            Campaign Name
                        </FieldLabel>
                        <Input
                            {...field}
                            id="form-rhf-demo-name"
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                        />
                        <FieldDescription>
                            Give your campaign a descriptive name to identify it
                        </FieldDescription>
                        {fieldState.invalid && (
                            <FieldError errors={[fieldState.error]} />
                        )}
                    </Field>
                )}
            />
            <Controller
                name="subject"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-subject">
                            Subject
                        </FieldLabel>
                        <Input
                            {...field}
                            id="form-rhf-demo-subject"
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                        />
                        {fieldState.invalid && (
                            <FieldError errors={[fieldState.error]} />
                        )}
                    </Field>
                )}
            />
            <Controller
                name="fromName"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor="form-rhf-demo-fromName">
                            From Name
                        </FieldLabel>
                        <Input
                            {...field}
                            id="form-rhf-demo-fromName"
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                        />
                        {fieldState.invalid && (
                            <FieldError errors={[fieldState.error]} />
                        )}
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
                            placeholder=""
                            autoComplete="off"
                        />
                        {fieldState.invalid && (
                            <FieldError errors={[fieldState.error]} />
                        )}
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
                        {fieldState.invalid && (
                            <FieldError errors={[fieldState.error]} />
                        )}
                    </Field>
                )}
            />
        </FieldGroup>
    )
}

export interface EmailSetupProps {
    form: UseFormReturn<z.infer<typeof setupFormEmail>>
    className?: string
}

function EmailSetup({ className, form }: EmailSetupProps) {
    const loadingSubjects = 3;
    const { subject, fromName, fromEmail } = form.watch()

    return (
        <>
            <div className={cn('bg-white border rounded-md shadow-sm w-full overflow-hidden', className)}>
                <div className="flex items-center gap-3 px-4 py-2 border-b">
                    <div className="flex items-center gap-3">
                        <input type="checkbox" className="h-4 w-4 rounded border-gray-300" />
                        <div className="flex items-center gap-2 text-gray-400">
                            <svg className="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M12 17.75L18.25 21l-1.5-7 5.25-4.75-7-.75L12 2 9 8.5l-7 .75L7.25 14l-1.5 7L12 17.75z" /></svg>
                            <svg className="h-4 w-4" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M9 5l7 7-7 7" /></svg>
                        </div>
                    </div>
                    <div className="w-2/5 flex items-center gap-1 truncate min-w-0">
                        <span className="font-semibold text-gray-900 whitespace-nowrap">{fromName}</span>
                        <span className="text-gray-400 whitespace-nowrap truncate">
                            &lt;{fromEmail}&gt;
                        </span>
                    </div>
                    <div className="w-3/5 font-medium text-sm truncate">
                        {subject}
                    </div>
                </div>

                <div>
                    {Array.from({ length: loadingSubjects }).map(() => (
                        <div className="flex items-center gap-3 px-4 py-2">
                            <div className="flex items-center gap-3">
                                <input type="checkbox" className="h-4 w-4 rounded border-gray-300" disabled />
                                <div className="flex items-center gap-2 text-gray-400">
                                    <svg className="h-4 w-4 opacity-40" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M12 17.75L18.25 21l-1.5-7 5.25-4.75-7-.75L12 2 9 8.5l-7 .75L7.25 14l-1.5 7L12 17.75z" /></svg>
                                    <svg className="h-4 w-4 opacity-40" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path d="M9 5l7 7-7 7" /></svg>
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
            <p className="text-center leading-7 mt-4 text-muted-foreground">Preview only, email clients may display differently.</p>
        </>
    )
}

const frameworks = [
    {
        value: "1234",
        label: "jeroen@cloudproud.nl",
    },
]

export function UserSelection() {
    const [open, setOpen] = useState(false)
    const [value, setValue] = useState("1234")

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    className="w-[200px] justify-between"
                >
                    {value
                        ? frameworks.find((framework) => framework.value === value)?.label
                        : "Select user..."}
                    <ChevronsUpDown className="opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[200px] p-0">
                <Command>
                    <CommandInput placeholder="Search user..." className="h-9" />
                    <CommandList>
                        <CommandEmpty>No user found.</CommandEmpty>
                        <CommandGroup>
                            {frameworks.map((framework) => (
                                <CommandItem
                                    key={framework.value}
                                    value={framework.value}
                                    onSelect={(currentValue) => {
                                        setValue(currentValue === value ? "" : currentValue)
                                        setOpen(false)
                                    }}
                                >
                                    {framework.label}
                                    <Check
                                        className={cn(
                                            "ml-auto",
                                            value === framework.value ? "opacity-100" : "opacity-0"
                                        )}
                                    />
                                </CommandItem>
                            ))}
                        </CommandGroup>
                    </CommandList>
                </Command>
            </PopoverContent>
        </Popover>
    )
}

export default function CampaignSetup() {
    const [project] = useContext(ProjectContext)
    // const { t } = useTranslation()
    // const [campaign, setCampaign] = useContext(CampaignContext)

    const form = useForm<z.infer<typeof setupFormEmail>>({
        resolver: zodResolver(setupFormEmail),
        defaultValues: {
            name: '',
            subject: 'You\'re off to a great start! {user.firstName}',
            fromName: 'Lunogram',
            fromEmail: '4b756f9e-87aa-40a0-9b57-05e23ef3126e@campaign.lunogram.com',
            replyTo: '',
        },
    })

    return (
        <div className="flex flex-1 bg-muted/20 overflow-hidden flex-row bg-muted/20">
            <div className="h-full overflow-y-auto w-2/5 bg-background p-8 border-r">
                <div className="mb-4">
                    <h1 className="text-2xl font-semibold">Setup</h1>
                    <p>Configure your email settings below.</p>
                </div>

                <EmailFormControl form={form} />
            </div>

            <div className="w-3/5 bg-background p-8 flex flex-col">
                <div className="flex items-center justify-between mb-6">
                    <div>
                        <UserSelection />
                    </div>

                    <a href={`/projects/${project?.id}/campaigns`}>
                        <Button variant="outline">Exit</Button>
                    </a>
                </div>
                <EmailSetup form={form} />
            </div>
        </div >
    )
}
