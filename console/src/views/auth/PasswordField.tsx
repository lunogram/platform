import { useState } from "react"
import { Eye, EyeOff, Lock } from "lucide-react"
import type { Control, FieldPath, FieldValues } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { Input } from "@/components/ui/input"
import {
    FormControl,
    FormDescription,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "@/components/ui/form"

interface PasswordFieldProps<T extends FieldValues> {
    control: Control<T>
    name: FieldPath<T>
    label: string
    description?: string
    autoComplete?: string
}

// PasswordField is the one place a password input is defined, so the reveal
// toggle and the autocomplete hint stay consistent across sign-in, registration,
// reset and change. The reveal is not a nicety: a long passphrase typed blind is
// a passphrase people shorten.
export default function PasswordField<T extends FieldValues>({
    control,
    name,
    label,
    description,
    autoComplete = "current-password",
}: PasswordFieldProps<T>) {
    const { t } = useTranslation()
    const [revealed, setRevealed] = useState(false)

    return (
        <FormField
            control={control}
            name={name}
            render={({ field }) => (
                <FormItem>
                    <FormLabel>{label}</FormLabel>
                    <FormControl>
                        <div className="relative">
                            <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                type={revealed ? "text" : "password"}
                                className="pl-9 pr-9"
                                autoComplete={autoComplete}
                                {...field}
                            />
                            <button
                                type="button"
                                onClick={() => setRevealed((shown) => !shown)}
                                aria-label={revealed ? t("password_hide") : t("password_show")}
                                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                            >
                                {revealed ? (
                                    <EyeOff className="h-4 w-4" />
                                ) : (
                                    <Eye className="h-4 w-4" />
                                )}
                            </button>
                        </div>
                    </FormControl>
                    {description && <FormDescription>{description}</FormDescription>}
                    <FormMessage />
                </FormItem>
            )}
        />
    )
}
