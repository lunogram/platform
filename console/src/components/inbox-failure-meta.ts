import type { TFunction } from "i18next"
import { ShieldCheck, TriangleAlert, type LucideIcon } from "lucide-react"

import type { components } from "../oapi/management.generated"

type InboxMessage = components["schemas"]["InboxMessage"]

export type InboxFailureKind = "suppressed" | "error"

export interface InboxFailureMeta {
    kind: InboxFailureKind
    label: string
    reason: string
    description: string
    icon: LucideIcon
}

export function getFailureMeta(
    failureReason: string | null | undefined,
    t: TFunction,
): InboxFailureMeta {
    switch (failureReason) {
        case "recipient_opted_out":
            return {
                kind: "suppressed",
                label: t("inbox_failure_suppressed", "Not sent"),
                reason: t("inbox_failure_recipient_opted_out", "Recipient opted out"),
                description: t(
                    "inbox_failure_recipient_opted_out_description",
                    "The recipient opted out of SMS, so this message was deliberately not sent. The opt-out was honoured and no action is needed.",
                ),
                icon: ShieldCheck,
            }
        case "invalid_recipient":
            return {
                kind: "error",
                label: t("inbox_failure_failed", "Failed"),
                reason: t("inbox_failure_invalid_recipient", "Invalid recipient"),
                description: t(
                    "inbox_failure_invalid_recipient_description",
                    "The recipient address was rejected as invalid, so the message could never be delivered. Correct the address on this profile before sending again.",
                ),
                icon: TriangleAlert,
            }
        case "sender_unregistered":
            return {
                kind: "error",
                label: t("inbox_failure_failed", "Failed"),
                reason: t("inbox_failure_sender_unregistered", "Sender not registered"),
                description: t(
                    "inbox_failure_sender_unregistered_description",
                    "The sender identity is not registered with the provider, so it may not send on this channel. Every message from this sender will fail until it is registered.",
                ),
                icon: TriangleAlert,
            }
        case "rate_limited":
            return {
                kind: "error",
                label: t("inbox_failure_failed", "Failed"),
                reason: t("inbox_failure_rate_limited", "Rate limited"),
                description: t(
                    "inbox_failure_rate_limited_description",
                    "The provider refused this message because the sending rate limit was exceeded, and it was given up on rather than retried.",
                ),
                icon: TriangleAlert,
            }
        default:
            return {
                kind: "error",
                label: t("inbox_failure_failed", "Failed"),
                reason: t("inbox_failure_unknown", "Unknown error"),
                description: t(
                    "inbox_failure_unknown_description",
                    "This message could not be delivered and the provider gave no reason. Check the provider logs for this message ID.",
                ),
                icon: TriangleAlert,
            }
    }
}

export function getMessageFailure(message: InboxMessage, t: TFunction): InboxFailureMeta | null {
    if (!message.failed_at) return null
    return getFailureMeta(message.failure_reason, t)
}

export function failureBadgeClassName(kind: InboxFailureKind) {
    return kind === "error"
        ? "border-0 bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
        : "border-0 bg-muted text-muted-foreground"
}

/**
 * Tailwind classes for the notice rendered inside the message preview. The
 * preview canvas paints its own light gradient in both themes, so these stay
 * light-mode only on purpose — `dark:` variants would render dark text on it.
 */
export function failureNoticeClassName(kind: InboxFailureKind) {
    return kind === "error"
        ? "border-red-200 bg-red-50 text-red-800"
        : "border-slate-200 bg-white text-slate-600"
}
