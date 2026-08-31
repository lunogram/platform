import { Link } from "react-router"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { CheckCircle2, Loader2, Mail } from "lucide-react"

import api from "../../api"
import { forgotPasswordSchema, type ForgotPasswordFormValues } from "@/validation/auth/password"
import AuthCard from "./AuthCard"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "@/components/ui/form"

export default function ForgotPassword() {
    const { t } = useTranslation()
    const [error, setError] = useState<string>()
    const [submitted, setSubmitted] = useState(false)
    const [isSubmitting, setIsSubmitting] = useState(false)

    const form = useForm<ForgotPasswordFormValues>({
        resolver: zodResolver(forgotPasswordSchema),
        defaultValues: { email: "" },
    })

    const handleRequest = async (data: ForgotPasswordFormValues) => {
        setIsSubmitting(true)
        setError(undefined)
        try {
            await api.auth.requestPasswordReset(data.email)
            setSubmitted(true)
        } catch (err) {
            const status = (err as { response?: { status?: number } })?.response?.status
            setError(status === 429 ? t("login_too_many_attempts") : t("forgot_password_failed"))
            setIsSubmitting(false)
        }
    }

    // Shown for every address, including ones with no account. The server
    // answers identically either way, and a confirmation that only appeared for
    // real accounts would give back exactly what that costs it.
    if (submitted) {
        return (
            <AuthCard
                title={t("forgot_password_sent")}
                description={t("forgot_password_sent_hint")}
            >
                <div className="flex justify-center py-2">
                    <CheckCircle2 className="h-10 w-10 text-muted-foreground" />
                </div>
                <Button asChild variant="outline" className="w-full">
                    <Link to="/login">{t("login_back_to_sign_in")}</Link>
                </Button>
            </AuthCard>
        )
    }

    return (
        <AuthCard
            title={t("forgot_password_title")}
            description={t("forgot_password_description")}
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
                <form onSubmit={form.handleSubmit(handleRequest)} className="space-y-4">
                    <FormField
                        control={form.control}
                        name="email"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t("email")}</FormLabel>
                                <FormControl>
                                    <div className="relative">
                                        <Mail className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                                        <Input
                                            type="email"
                                            placeholder="name@example.com"
                                            className="pl-9"
                                            autoComplete="username"
                                            {...field}
                                        />
                                    </div>
                                </FormControl>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    <Button type="submit" className="w-full" disabled={isSubmitting}>
                        {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        {t("forgot_password_submit")}
                    </Button>
                </form>
            </Form>
        </AuthCard>
    )
}
