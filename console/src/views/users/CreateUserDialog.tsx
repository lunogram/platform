import { useMemo, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Check } from "lucide-react"
import { cn } from "../../utils"
import type { User } from "../../types"
import { createUserSchema, type CreateUserFormValues } from "@/validation/users/create-user"
import { useCreateUser } from "./queries"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import { LocalePicker } from "@/components/locale/picker"

// eslint-disable-next-line @typescript-eslint/no-namespace
export declare namespace Intl {
    type Key = "calendar" | "collation" | "currency" | "numberingSystem" | "timeZone" | "unit"
    function supportedValuesOf(input: Key): string[]

    interface DateTimeFormat {
        format(date?: Date | number): string

        resolvedOptions(): ResolvedDateTimeFormatOptions
    }

    interface ResolvedDateTimeFormatOptions {
        locale: string
        timeZone: string
        timeZoneName?: string
    }

    // eslint-disable-next-line no-var
    var DateTimeFormat: {
        new (locales?: string | string[]): DateTimeFormat
        (locales?: string | string[]): DateTimeFormat
    }
}

interface CreateUserDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

export function CreateUserDialog({
    open: isCreateOpen,
    onOpenChange: setIsCreateOpen,
}: CreateUserDialogProps) {
    const { t } = useTranslation()
    const createUserMutation = useCreateUser()
    const isCreating = createUserMutation.isPending
    const [isNewUserTimezoneOpen, setIsNewUserTimezoneOpen] = useState(false)
    const timezones = useMemo(() => Intl.supportedValuesOf("timeZone"), [])
    const [newUserData, setNewUserData] = useState<Record<string, unknown>>({})

    const form = useForm<CreateUserFormValues>({
        resolver: zodResolver(createUserSchema),
        defaultValues: {
            external_id: "",
            email: "",
            phone: "",
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            locale: navigator.languages[0]?.split("-")[0] ?? "en",
        },
    })

    // The dialog stays open on failure; the mutation cache reports the error.
    const createUser = (data: CreateUserFormValues) => {
        if (!data.external_id?.trim() && !data.email) return

        const newUser: User = {
            identifier: data.external_id?.trim()
                ? [{ source: "default", external_id: data.external_id.trim() }]
                : [{ source: "anonymous", external_id: crypto.randomUUID() }],
            email: data.email?.trim() || undefined,
            phone: data.phone?.trim() || undefined,
            timezone: data.timezone?.trim() || undefined,
            locale: data.locale?.trim() || undefined,
            data: newUserData,
        } as User

        createUserMutation.mutate(newUser, {
            onSuccess: () => {
                setIsCreateOpen(false)
                form.reset()
                setNewUserData({})
            },
        })
    }

    return (
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
            <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>{t("create_user")}</DialogTitle>
                    <DialogDescription>
                        {t(
                            "create_user_description",
                            "Create a new user to track and engage with.",
                        )}
                    </DialogDescription>
                </DialogHeader>
                <form id="create-user-form" onSubmit={form.handleSubmit(createUser)}>
                    <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                            <Label htmlFor="external_id">{t("identifier", "Identifier")}</Label>
                            <Input
                                id="external_id"
                                placeholder={t("enter_identifier", "e.g., usr_123")}
                                {...form.register("external_id")}
                            />
                            {form.formState.errors.external_id && (
                                <p className="text-sm text-destructive">
                                    {form.formState.errors.external_id.message}
                                </p>
                            )}
                        </div>
                        <div className="grid sm:grid-cols-2 gap-4">
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="email">{t("email")}</Label>
                                <Input
                                    id="email"
                                    type="email"
                                    placeholder={t("enter_email", "e.g., john@example.com")}
                                    {...form.register("email")}
                                />
                                {form.formState.errors.email && (
                                    <p className="text-sm text-destructive">
                                        {form.formState.errors.email.message}
                                    </p>
                                )}
                            </div>
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="phone">{t("phone")}</Label>
                                <Input
                                    id="phone"
                                    placeholder={t("enter_phone", "e.g., +1 555 0123")}
                                    {...form.register("phone")}
                                />
                                {form.formState.errors.phone && (
                                    <p className="text-sm text-destructive">
                                        {form.formState.errors.phone.message}
                                    </p>
                                )}
                            </div>
                        </div>
                        <div className="grid sm:grid-cols-2 gap-4">
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="timezone">{t("timezone")}</Label>
                                <Controller
                                    control={form.control}
                                    name="timezone"
                                    render={({ field }) => (
                                        <Popover
                                            open={isNewUserTimezoneOpen}
                                            onOpenChange={setIsNewUserTimezoneOpen}
                                        >
                                            <PopoverTrigger asChild>
                                                <Button
                                                    variant="outline"
                                                    role="combobox"
                                                    id="timezone"
                                                    className="justify-between"
                                                >
                                                    {field.value ||
                                                        t("select_timezone", "Select timezone")}
                                                </Button>
                                            </PopoverTrigger>
                                            <PopoverContent className="w-72 p-0" align="start">
                                                <Command>
                                                    <CommandInput
                                                        placeholder={t(
                                                            "search_timezone",
                                                            "Search timezone...",
                                                        )}
                                                    />
                                                    <CommandList>
                                                        <CommandEmpty>
                                                            {t(
                                                                "no_timezone_found",
                                                                "No timezone found.",
                                                            )}
                                                        </CommandEmpty>
                                                        <CommandGroup>
                                                            {timezones.map((tz) => (
                                                                <CommandItem
                                                                    key={tz}
                                                                    value={tz}
                                                                    onSelect={() => {
                                                                        field.onChange(tz)
                                                                        setIsNewUserTimezoneOpen(
                                                                            false,
                                                                        )
                                                                    }}
                                                                >
                                                                    <Check
                                                                        className={cn(
                                                                            "mr-2 h-4 w-4",
                                                                            field.value === tz
                                                                                ? "opacity-100"
                                                                                : "opacity-0",
                                                                        )}
                                                                    />
                                                                    {tz}
                                                                </CommandItem>
                                                            ))}
                                                        </CommandGroup>
                                                    </CommandList>
                                                </Command>
                                            </PopoverContent>
                                        </Popover>
                                    )}
                                />
                            </div>
                            <div className="grid gap-2 content-start">
                                <Label>{t("locale.singular")}</Label>
                                <Controller
                                    control={form.control}
                                    name="locale"
                                    render={({ field }) => (
                                        <LocalePicker
                                            value={field.value ?? ""}
                                            onChange={field.onChange}
                                        />
                                    )}
                                />
                            </div>
                        </div>
                        <div className="grid gap-2">
                            <Label>{t("data", "Data")}</Label>
                            <AttributeEditor
                                value={newUserData}
                                onChange={setNewUserData}
                                emptyTitle={t("no_data", "No data")}
                                emptyDescription={t(
                                    "no_data_description",
                                    "Add custom attributes to this user.",
                                )}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIsCreateOpen(false)}
                            disabled={isCreating}
                        >
                            {t("cancel")}
                        </Button>
                        <Button type="submit" disabled={isCreating}>
                            {isCreating ? t("creating") : t("create")}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}
