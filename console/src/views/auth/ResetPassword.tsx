import { Link, useSearchParams } from "react-router"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { CheckCircle2, Loader2 } from "lucide-react"

import api from "../../api"
import {
    resetPasswordSchema,
    type ResetPasswordFormValues,
} from "@/validation/auth/password"
import AuthCard from "./AuthCard"
import PasswordField from "./PasswordField"

import { Button } from "@/components/ui/button"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Form } from "@/components/ui/form"

export default function ResetPassword() {
    const { t } = useTranslation()
    const [searchParams] = useSearchParams()
    const [error, setError] = useState<string>()
    const [done, setDone] = useState(false)
    const [isSubmitting, setIsSubmitting] = useState(false)
    const token = searchParams.get("token") ?? ""

    const form = useForm<ResetPasswordFormValues>({
        resolver: zodResolver(resetPasswordSchema),
        defaultValues: { password: "", confirm: "" },
    })

    const handleReset = async (data: ResetPasswordFormValues) => {
        setIsSubmitting(true)
        setError(undefined)
        try {
            await api.auth.confirmPasswordReset(token, data.password)
            setDone(true)
        } catch (err) {
            const response = (err as { response?: { status?: number; data?: { detail?: string } } })
                ?.response
            setError(response?.data?.detail ?? t("reset_password_failed"))
            setIsSubmitting(false)
        }
    }

    if (!token) {
        return (
            <AuthCard title={t("reset_password_title")} description={t("reset_password_no_token")}>
                <Button asChild variant="outline" className="w-full">
                    <Link to="/forgot-password">{t("forgot_password_submit")}</Link>
                </Button>
            </AuthCard>
        )
    }

    // Completing a reset ends every session the account had, including any the
    // person doing the reset was holding, so the only way on is a fresh sign-in.
    if (done) {
        return (
            <AuthCard title={t("reset_password_done")} description={t("reset_password_done_hint")}>
                <div className="flex justify-center py-2">
                    <CheckCircle2 className="h-10 w-10 text-muted-foreground" />
                </div>
                <Button asChild className="w-full">
                    <Link to="/login">{t("login_back_to_sign_in")}</Link>
                </Button>
            </AuthCard>
        )
    }

    return (
        <AuthCard
            title={t("reset_password_title")}
            description={t("reset_password_description")}
            footer={
                <Link to="/login" className="underline underline-offset-4">
                    {t("login_back_to_sign_in")}
                </Link>
            }
        >
            {error && (
                <Alert variant="destructive">
                    <AlertTitle>{t("error")}</AlertTitle>
                    <AlertDescription>{error}</AlertDescription>
                </Alert>
            )}

            <Form {...form}>
                <form onSubmit={form.handleSubmit(handleReset)} className="space-y-4">
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

                    <Button type="submit" className="w-full" disabled={isSubmitting}>
                        {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        {t("reset_password_submit")}
                    </Button>
                </form>
            </Form>
        </AuthCard>
    )
}
