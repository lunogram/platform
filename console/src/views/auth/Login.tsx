import { Link, useSearchParams } from "react-router"
import { SignIn } from "@clerk/clerk-react"
import { useEffect, useState, useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Loader2, Mail, ArrowLeft } from "lucide-react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { loginSchema } from "@/validation/auth/login"

import api from "../../api"
import { type AuthDriver, type SsoDriver, AUTH_DRIVERS } from "../../types"
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

// The callback bounces a failed federated login back here with a coarse reason.
// It is deliberately coarse: enough to say something true, never enough to
// describe the deployment's identity provider to a stranger.
const SSO_ERRORS: Record<string, string> = {
    expired: "sso_error_expired",
    denied: "sso_error_denied",
    domain: "sso_error_domain",
    email: "sso_error_email",
    transient: "sso_error_transient",
    exchange: "sso_error_failed",
    failed: "sso_error_failed",
}

const SUPPORTED_DRIVERS: AuthDriver[] = [
    AUTH_DRIVERS.BASIC,
    AUTH_DRIVERS.CLERK,
    AUTH_DRIVERS.OIDC,
    AUTH_DRIVERS.SAML,
]

// The drivers that start a login by navigating away to an identity provider.
const SSO_DRIVERS: SsoDriver[] = [AUTH_DRIVERS.OIDC, AUTH_DRIVERS.SAML]

// A deployment may enable both protocols, but "sign in with OpenID Connect" and
// "sign in with SAML" is not a choice anybody wants to make: which protocol a
// directory speaks is the operator's problem, not the problem of the person
// signing in. The two drivers therefore collapse into one choice on the method
// picker, and the providers behind them into one list of buttons that carry
// their own protocol.
const SSO_CHOICE = "sso" as const

type DriverChoice = AuthDriver | typeof SSO_CHOICE

interface SsoProvider {
    id: string
    name: string
    driver: SsoDriver
}

// driverChoices replaces however many single sign-on drivers are enabled with
// the single collapsed choice, keeping the operator's declared order otherwise.
function driverChoices(drivers: AuthDriver[]): DriverChoice[] {
    const choices: DriverChoice[] = []
    for (const driver of drivers) {
        const choice: DriverChoice = SSO_DRIVERS.includes(driver as SsoDriver) ? SSO_CHOICE : driver
        if (!choices.includes(choice)) choices.push(choice)
    }
    return choices
}

export default function Login() {
    const { t } = useTranslation()
    const [searchParams] = useSearchParams()
    const [drivers, setDrivers] = useState<AuthDriver[]>()
    const [selectedDriver, setSelectedDriver] = useState<DriverChoice>()
    const [error, setError] = useState<string>()
    const [isSubmitting, setIsSubmitting] = useState(false)
    const [ssoProviders, setSsoProviders] = useState<SsoProvider[]>()
    const redirect = validateRedirect(searchParams.get("r"))

    const form = useForm<LoginFormValues>({
        resolver: zodResolver(loginSchema),
        defaultValues: {
            email: "",
            password: "",
        },
    })

    const handleSelectDriver = useCallback((driver: DriverChoice) => {
        setSelectedDriver(driver)
        setError(undefined)
    }, [])

    const handleCredentialAuth = async (data: LoginFormValues) => {
        if (!selectedDriver) return

        setIsSubmitting(true)
        try {
            await api.auth.basicAuth(data.email, data.password)
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
        const failure = searchParams.get("sso_error")
        if (failure) {
            setError(t(SSO_ERRORS[failure] ?? "sso_error_failed"))
        }
    }, [searchParams, t])

    useEffect(() => {
        api.auth
            .methods()
            .then((methods) => {
                const supportedDrivers = methods.filter((driver) =>
                    SUPPORTED_DRIVERS.includes(driver),
                )
                setDrivers(supportedDrivers)
                const choices = driverChoices(supportedDrivers)
                // Selected directly rather than through handleSelectDriver,
                // which clears the error. On a deployment offering only single
                // sign-on this runs right after a failed callback set one, and
                // clearing it would leave the person looking at the button they
                // just came back from with no explanation.
                if (choices.length === 1) {
                    setSelectedDriver(choices[0])
                }
            })
            .catch((err) => {
                console.error("Failed to fetch auth methods:", err)
                setError(t("login_methods_error"))
            })
    }, [t])

    // Fetched only for the protocols the deployment says it offers, so a
    // deployment without single sign-on makes no call that would 404, and one
    // with only SAML never asks for OpenID Connect providers.
    useEffect(() => {
        if (!drivers) return
        const enabled = SSO_DRIVERS.filter((driver) => drivers.includes(driver))
        if (enabled.length === 0) return

        Promise.all(
            enabled.map(async (driver) =>
                (await api.auth.ssoProviders(driver)).map((provider) => ({
                    ...provider,
                    driver,
                })),
            ),
        )
            .then((lists) => setSsoProviders(lists.flat()))
            .catch((err) => {
                console.error("Failed to fetch sso providers:", err)
                // Leaving this undefined would spin the loader forever behind
                // the error message. There is nothing to offer, so say so by
                // having nothing to render.
                setSsoProviders([])
                setError(t("sso_error_failed"))
            })
    }, [drivers, t])

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
                {driverChoices(drivers).length > 1 && (
                    <Button variant="ghost" onClick={() => setSelectedDriver(undefined)}>
                        <ArrowLeft className="mr-2 h-4 w-4" />
                        {t("back")}
                    </Button>
                )}
            </div>
        )
    }

    const choices = driverChoices(drivers)
    const isCredentialDriver = selectedDriver === AUTH_DRIVERS.BASIC
    const offersAccounts = isCredentialDriver
    const isSsoDriver = selectedDriver === SSO_CHOICE

    return (
        <AuthCard
            title={t("welcome")}
            description={
                selectedDriver
                    ? isSsoDriver
                        ? ssoProviders && ssoProviders.length > 1
                            ? t("sso_choose_provider")
                            : t("sso_instructions")
                        : t("login_basic_instructions")
                    : t("login_select_method")
            }
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
                    {choices.map((choice) => (
                        <Button
                            key={choice}
                            variant="outline"
                            className="w-full"
                            onClick={() => handleSelectDriver(choice)}
                        >
                            {t(`auth_driver_${choice}`)}
                        </Button>
                    ))}
                </div>
            )}

            {isSsoDriver && (
                <div className="space-y-4">
                    {!ssoProviders && (
                        <div className="flex justify-center py-4">
                            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                        </div>
                    )}

                    {ssoProviders?.map((provider) => (
                        <Button
                            key={provider.id}
                            type="button"
                            className="w-full"
                            disabled={isSubmitting}
                            onClick={() => {
                                setIsSubmitting(true)
                                api.auth.ssoStart(provider.driver, provider.id, redirect)
                            }}
                        >
                            {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                            {ssoProviders.length === 1 ? t("sso_continue") : provider.name}
                        </Button>
                    ))}

                    {choices.length > 1 && (
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

                        {choices.length > 1 && (
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
