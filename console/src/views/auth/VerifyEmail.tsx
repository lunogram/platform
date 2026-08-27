import { Link, useSearchParams } from "react-router"
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { CheckCircle2, Loader2, XCircle } from "lucide-react"

import api from "../../api"
import AuthCard from "./AuthCard"

import { Button } from "@/components/ui/button"

type VerifyState = "verifying" | "verified" | "failed"

export default function VerifyEmail() {
    const { t } = useTranslation()
    const [searchParams] = useSearchParams()
    const token = searchParams.get("token") ?? ""
    const [state, setState] = useState<VerifyState>(token ? "verifying" : "failed")

    // The token is single use, so a second redemption always fails. React runs
    // effects twice in development, which would otherwise turn every successful
    // confirmation into a failure on the screen.
    const redeemed = useRef(false)

    useEffect(() => {
        if (!token || redeemed.current) return
        redeemed.current = true

        api.auth
            .verifyEmail(token)
            .then(() => setState("verified"))
            .catch(() => setState("failed"))
    }, [token])

    if (state === "verifying") {
        return (
            <AuthCard title={t("verify_email_title")}>
                <div className="flex flex-col items-center gap-3 py-4">
                    <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                    <p className="text-sm text-muted-foreground">{t("loading")}</p>
                </div>
            </AuthCard>
        )
    }

    if (state === "verified") {
        return (
            <AuthCard title={t("verify_email_done")} description={t("verify_email_done_hint")}>
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
        <AuthCard title={t("verify_email_failed")} description={t("verify_email_failed_hint")}>
            <div className="flex justify-center py-2">
                <XCircle className="h-10 w-10 text-muted-foreground" />
            </div>
            <Button asChild variant="outline" className="w-full">
                <Link to="/login">{t("login_back_to_sign_in")}</Link>
            </Button>
        </AuthCard>
    )
}
