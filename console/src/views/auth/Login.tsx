import { Link, useSearchParams } from "react-router"
import { SignIn } from "@clerk/clerk-react"
import { useEffect, useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Loader2, Mail, ArrowLeft } from "lucide-react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { loginSchema } from "@/validation/auth/login"

import api from "../../api"
import { type AuthDriver, AUTH_DRIVERS } from "../../types"
import { validateRedirect } from "@/lib/validate-redirect"
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

interface LoginFormValues {
    email: string
    password: string
}

const SUPPORTED_DRIVERS: AuthDriver[] = [
    AUTH_DRIVERS.BASIC,
    AUTH_DRIVERS.PASSWORD,
    AUTH_DRIVERS.CLERK,
]

// The basic and password drivers submit the same form to different callbacks:
// basic proves the single credential in the deployment's configuration, password
// proves a per-admin credential the deployment stores. Only the second has
// accounts, so only it offers registration and recovery.
const CREDENTIAL_DRIVERS: AuthDriver[] = [AUTH_DRIVERS.BASIC, AUTH_DRIVERS.PASSWORD]

export default function Login() {
    const { t } = useTranslation()
    const [searchParams] = useSearchParams()
    const [drivers, setDrivers] = useState<AuthDriver[]>()
    const [selectedDriver, setSelectedDriver] = useState<AuthDriver>()
    const [error, setError] = useState<string>()
    const [isSubmitting, setIsSubmitting] = useState(false)
    const redirect = validateRedirect(searchParams.get("r"))

    const form = useForm<LoginFormValues>({
        resolver: zodResolver(loginSchema),
        defaultValues: {
            email: "",
            password: "",
        },
    })

    const handleSelectDriver = useCallback((driver: AuthDriver) => {
        setSelectedDriver(driver)
        setError(undefined)
    }, [])

    const handleCredentialAuth = async (data: LoginFormValues) => {
        if (!selectedDriver) return

        setIsSubmitting(true)
        try {
            if (selectedDriver === AUTH_DRIVERS.PASSWORD) {
                await api.auth.passwordAuth(data.email, data.password)
            } else {
                await api.auth.basicAuth(data.email, data.password)
            }
            window.location.href = redirect
        } catch (err) {
            // A wrong password and an address with no account answer
            // identically, so there is only one message to show.
            const status = (err as { response?: { status?: number } })?.response?.status
            setError(status === 429 ? t("login_too_many_attempts") : t("login_invalid_credentials"))
            setIsSubmitting(false)
        }
    }

    useEffect(() => {
        api.auth
            .methods()
            .then((methods) => {
                const supportedDrivers = methods.filter((driver) =>
                    SUPPORTED_DRIVERS.includes(driver),
                )
                setDrivers(supportedDrivers)
                if (supportedDrivers.length === 1) {
                    handleSelectDriver(supportedDrivers[0])
                }
            })
            .catch((err) => {
                console.error("Failed to fetch auth methods:", err)
                setError(t("login_methods_error"))
            })
    }, [handleSelectDriver, t])

    // Loading state
    if (!drivers || drivers.length === 0) {
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

    // Clerk auth - render standalone without Card wrapper
    if (selectedDriver === AUTH_DRIVERS.CLERK) {
        return (
            <div className="min-h-screen flex flex-col items-center justify-center bg-muted/40 p-4 gap-4">
                <SignIn
                    forceRedirectUrl={`/login/clerk/callback?r=${encodeURIComponent(redirect)}`}
                />
                {drivers.length > 1 && (
                    <Button variant="ghost" onClick={() => setSelectedDriver(undefined)}>
                        <ArrowLeft className="mr-2 h-4 w-4" />
                        {t("back")}
                    </Button>
                )}
            </div>
        )
    }

    const isCredentialDriver = selectedDriver && CREDENTIAL_DRIVERS.includes(selectedDriver)
    const offersAccounts = selectedDriver === AUTH_DRIVERS.PASSWORD

    return (
        <AuthCard
            title={t("welcome")}
            description={selectedDriver ? t("login_basic_instructions") : t("login_select_method")}
            footer={
                offersAccounts ? (
                    <Link to="/register" className="underline underline-offset-4">
                        {t("login_no_account")}
                    </Link>
                ) : undefined
            }
        >
            {error && (
                <Alert variant="destructive">
                    <AlertTitle>{t("error")}</AlertTitle>
                    <AlertDescription>{error}</AlertDescription>
                </Alert>
            )}

            {!selectedDriver && (
                <div className="flex flex-col gap-3">
                    {drivers.map((driver) => (
                        <Button
                            key={driver}
                            variant="outline"
                            className="w-full"
                            onClick={() => handleSelectDriver(driver)}
                        >
                            {t(`auth_driver_${driver}`)}
                        </Button>
                    ))}
                </div>
            )}

            {isCredentialDriver && (
                <Form {...form}>
                    <form onSubmit={form.handleSubmit(handleCredentialAuth)} className="space-y-4">
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
                            autoComplete="current-password"
                        />

                        {offersAccounts && (
                            <div className="text-right">
                                <Link
                                    to="/forgot-password"
                                    className="text-sm text-muted-foreground underline underline-offset-4"
                                >
                                    {t("login_forgot_password")}
                                </Link>
                            </div>
                        )}

                        <Button type="submit" className="w-full" disabled={isSubmitting}>
                            {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                            {t("submit")}
                        </Button>

                        {drivers.length > 1 && (
                            <Button
                                type="button"
                                variant="ghost"
                                className="w-full"
                                onClick={() => setSelectedDriver(undefined)}
                            >
                                <ArrowLeft className="mr-2 h-4 w-4" />
                                {t("back")}
                            </Button>
                        )}
                    </form>
                </Form>
            )}
        </AuthCard>
    )
}
