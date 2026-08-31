import { Link, useSearchParams } from "react-router"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { CheckCircle2, Loader2, XCircle } from "lucide-react"

import api from "../../api"
import AuthCard from "./AuthCard"

import { Button } from "@/components/ui/button"

type VerifyState = "ready" | "verifying" | "verified" | "failed"

export default function VerifyEmail() {
    const { t } = useTranslation()
    const [searchParams] = useSearchParams()
    const token = searchParams.get("token") ?? ""
    const [state, setState] = useState<VerifyState>(token ? "ready" : "failed")

    // Redeeming on render would let anything that merely opens the link spend
    // the token: mail security scanners and link-preview fetchers follow URLs
    // out of a mailbox, and one of them reaching this page first would both
    // burn the recipient's link and confirm an address nobody had demonstrated
    // control of. A click is what distinguishes a person from a crawler.
    const confirm = async () => {
        setState("verifying")
        try {
            await api.auth.verifyEmail(token)
            setState("verified")
        } catch {
            setState("failed")
        }
    }

    if (state === "ready" || state === "verifying") {
        return (
            <AuthCard title={t("verify_email_title")} description={t("verify_email_prompt")}>
                <Button className="w-full" onClick={confirm} disabled={state === "verifying"}>
                    {state === "verifying" ? (
                        <>
                            <Loader2 className="h-4 w-4 animate-spin" />
                            {t("verify_email_working")}
                        </>
                    ) : (
                        t("verify_email_action")
                    )}
                </Button>
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
