import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { Loader2 } from "lucide-react"

import api from "@/api"
import {
    changePasswordSchema,
    type ChangePasswordFormValues,
} from "@/validation/auth/password"
import PasswordField from "@/views/auth/PasswordField"

import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Form } from "@/components/ui/form"
import { toast } from "sonner"

interface ChangePasswordDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
}

export function ChangePasswordDialog({ open, onOpenChange }: ChangePasswordDialogProps) {
    const { t } = useTranslation()
    const [error, setError] = useState<string>()
    const [isSubmitting, setIsSubmitting] = useState(false)

    const form = useForm<ChangePasswordFormValues>({
        resolver: zodResolver(changePasswordSchema),
        defaultValues: { current_password: "", password: "", confirm: "" },
    })

    const handleChange = async (data: ChangePasswordFormValues) => {
        setIsSubmitting(true)
        setError(undefined)
        try {
            await api.auth.changePassword(data.current_password, data.password)
            form.reset()
            onOpenChange(false)
            // Every other session was just ended, which is the part worth
            // saying out loud: somebody changing their password wants to know
            // whoever else was signed in is now signed out.
            toast.success(t("change_password_done"))
        } catch (err) {
            const response = (err as { response?: { status?: number; data?: { detail?: string } } })
                ?.response
            setError(
                response?.status === 403
                    ? t("change_password_wrong_current")
                    : (response?.data?.detail ?? t("change_password_failed")),
            )
        } finally {
            setIsSubmitting(false)
        }
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>{t("change_password")}</DialogTitle>
                    <DialogDescription>{t("change_password_description")}</DialogDescription>
                </DialogHeader>

                {error && (
                    <Alert variant="destructive">
                        <AlertTitle>{t("error")}</AlertTitle>
                        <AlertDescription>{error}</AlertDescription>
                    </Alert>
                )}

                <Form {...form}>
                    <form onSubmit={form.handleSubmit(handleChange)} className="space-y-4">
                        <PasswordField
                            control={form.control}
                            name="current_password"
                            label={t("password_current")}
                            autoComplete="current-password"
                        />
                        <PasswordField
                            control={form.control}
                            name="password"
                            label={t("password_new")}
                            description={t("password_requirements")}
                            autoComplete="new-password"
                        />
                        <PasswordField
                            control={form.control}
                            name="confirm"
                            label={t("password_confirm")}
                            autoComplete="new-password"
                        />

                        <DialogFooter>
                            <Button
                                type="button"
                                variant="outline"
                                onClick={() => onOpenChange(false)}
                            >
                                {t("cancel")}
                            </Button>
                            <Button type="submit" disabled={isSubmitting}>
                                {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                {t("change_password")}
                            </Button>
                        </DialogFooter>
                    </form>
                </Form>
            </DialogContent>
        </Dialog>
    )
}
