import { Link, useSearchParams } from "react-router"
import { SignUp } from "@clerk/clerk-react"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { CheckCircle2, Loader2, Mail } from "lucide-react"

import api from "../../api"
import { type AuthDriver, AUTH_DRIVERS } from "../../types"
import { validateRedirect } from "@/lib/validate-redirect"
import {
    MIN_PASSWORD_LENGTH,
    registerSchema,
    type RegisterFormValues,
} from "@/validation/auth/password"
import AuthCard from "./AuthCard"
import PasswordField from "./PasswordField"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardContent } from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "@/components/ui/form"

export default function Register() {
    const { t } = useTranslation()
    const [searchParams] = useSearchParams()
    const [drivers, setDrivers] = useState<AuthDriver[]>()
    const [error, setError] = useState<string>()
    const [submitted, setSubmitted] = useState(false)
    const [isSubmitting, setIsSubmitting] = useState(false)
    const redirect = validateRedirect(searchParams.get("r"))

    const form = useForm<RegisterFormValues>({
        resolver: zodResolver(registerSchema),
        defaultValues: { email: "", password: "", first_name: "", last_name: "" },
    })

    useEffect(() => {
        api.auth
            .methods()
            .then(setDrivers)
            .catch((err) => {
                console.error("Failed to fetch auth methods:", err)
                setError(t("login_methods_error"))
            })
    }, [t])

    const handleRegister = async (data: RegisterFormValues) => {
        setIsSubmitting(true)
        setError(undefined)
        try {
            await api.auth.register({
                email: data.email,
                password: data.password,
                first_name: data.first_name || undefined,
                last_name: data.last_name || undefined,
            })
            setSubmitted(true)
        } catch (err) {
            const response = (err as { response?: { status?: number; data?: { detail?: string } } })
                ?.response
            if (response?.status === 429) {
                setError(t("login_too_many_attempts"))
            } else if (response?.status === 404) {
                setError(t("register_closed"))
            } else {
                setError(response?.data?.detail ?? t("register_failed"))
            }
            setIsSubmitting(false)
        }
    }

    if (!drivers) {
        return (
            <div className="min-h-screen flex items-center justify-center bg-muted/40">
                <Card className="w-full max-w-sm">
                    <CardContent className="flex flex-col items-center justify-center py-12">
                        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                        <p className="mt-4 text-sm text-muted-foreground">{t("loading")}</p>
                    </CardContent>
                </Card>
            </div>
        )
    }

    // Clerk owns its own sign-up experience, so the console hands over to it
    // rather than reimplementing one. This used to be unconditional, which meant
    // a deployment that had never configured Clerk still got its widget on
    // /register and no way to create an account at all.
    if (drivers.includes(AUTH_DRIVERS.CLERK) && !drivers.includes(AUTH_DRIVERS.BASIC)) {
        return (
            <div className="min-h-screen flex flex-col items-center justify-center bg-muted/40 p-4 gap-4">
                <SignUp
                    forceRedirectUrl={`/login/clerk/callback?r=${encodeURIComponent(redirect)}`}
                />
            </div>
        )
    }

    if (!drivers.includes(AUTH_DRIVERS.BASIC)) {
        return (
            <AuthCard title={t("register_title")} description={t("register_closed")}>
                <Button asChild variant="outline" className="w-full">
                    <Link to="/login">{t("login_back_to_sign_in")}</Link>
                </Button>
            </AuthCard>
        )
    }

    // The confirmation is deliberately the same whether the address was free or
    // already taken: the server answers identically, and saying anything more
    // specific here would undo that.
    if (submitted) {
        return (
            <AuthCard
                title={t("register_check_inbox")}
                description={t("register_check_inbox_hint")}
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
            title={t("register_title")}
            description={t("register_description")}
            footer={
                <Link to="/login" className="underline underline-offset-4">
                    {t("register_have_account")}
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
                <form onSubmit={form.handleSubmit(handleRegister)} className="space-y-4">
                    <div className="grid grid-cols-2 gap-3">
                        <FormField
                            control={form.control}
                            name="first_name"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>{t("first_name")}</FormLabel>
                                    <FormControl>
                                        <Input autoComplete="given-name" {...field} />
                                    </FormControl>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                        <FormField
                            control={form.control}
                            name="last_name"
                            render={({ field }) => (
                                <FormItem>
                                    <FormLabel>{t("last_name")}</FormLabel>
                                    <FormControl>
                                        <Input autoComplete="family-name" {...field} />
                                    </FormControl>
                                    <FormMessage />
                                </FormItem>
                            )}
                        />
                    </div>

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

                    <PasswordField
                        control={form.control}
                        name="password"
                        label={t("password")}
                        description={t("password_requirements", { count: MIN_PASSWORD_LENGTH })}
                        autoComplete="new-password"
                    />

                    <Button type="submit" className="w-full" disabled={isSubmitting}>
                        {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                        {t("register_submit")}
                    </Button>
                </form>
            </Form>
        </AuthCard>
    )
}
