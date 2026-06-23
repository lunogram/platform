import { useState } from "react"
import { Check, Copy, CircleCheck, TriangleAlert } from "lucide-react"

import { type Client } from "./model"
import { RequestPreview } from "./RequestPreview"

import { Button } from "@/components/ui/button"

// ClientCreated is the one-time confirmation shown right after a client is made:
// the generated secret (revealed once), and a ready-to-run example.
export function ClientCreated({
    client,
    secret,
    onDone,
    onCreateAnother,
}: {
    client: Client
    secret: string | null
    onDone: () => void
    onCreateAnother: () => void
}) {
    const [copied, setCopied] = useState(false)
    const copy = () => {
        if (!secret) return
        void navigator.clipboard?.writeText(secret)
        setCopied(true)
        window.setTimeout(() => setCopied(false), 1500)
    }

    // A secret is only ever issued for API keys. Trusted issuers and sessions
    // authenticate without a per-client secret, so explain how each is used.
    const noSecretMessage =
        client.identity.type === "session"
            ? "Sessions are minted from your backend by calling the sessions endpoint with a secret API key — there's no separate secret for this client."
            : "This client verifies tokens from your identity provider, so there's no secret to store."

    return (
        <div className="mx-auto flex max-w-2xl flex-col gap-8 py-4">
            <div className="flex flex-col items-center gap-3 text-center">
                <span className="flex h-12 w-12 items-center justify-center rounded-full bg-green-soft text-green-hard">
                    <CircleCheck className="h-6 w-6" strokeWidth={2} />
                </span>
                <div className="grid gap-1">
                    <h2 className="text-2xl font-semibold tracking-tight">Client created</h2>
                    <p className="text-sm text-ink-soft">
                        <span className="font-medium text-foreground">{client.name}</span> is ready
                        to use.
                    </p>
                </div>
            </div>

            {secret ? (
                <div className="grid gap-2">
                    <div className="flex items-center gap-1.5 text-sm font-medium">
                        Secret key
                    </div>
                    <div className="flex items-center gap-2 rounded-lg border bg-surface-muted px-3 py-2.5">
                        <code className="flex-1 truncate font-mono text-sm">{secret}</code>
                        <Button
                            variant="outline"
                            size="sm"
                            className="shrink-0 gap-1.5 bg-card"
                            onClick={copy}
                        >
                            {copied ? (
                                <>
                                    <Check className="h-3.5 w-3.5" /> Copied
                                </>
                            ) : (
                                <>
                                    <Copy className="h-3.5 w-3.5" /> Copy
                                </>
                            )}
                        </Button>
                    </div>
                    <p className="flex items-center gap-1.5 text-xs text-amber-hard">
                        <TriangleAlert className="h-3.5 w-3.5 shrink-0" strokeWidth={2} />
                        Copy this now — for security, it won't be shown again.
                    </p>
                </div>
            ) : (
                <p className="rounded-lg border bg-surface-soft px-4 py-3 text-sm text-ink-soft">
                    {noSecretMessage}
                </p>
            )}

            <div className="grid gap-2">
                <span className="text-sm font-medium">Quick start</span>
                <RequestPreview client={client} secret={secret ?? undefined} />
            </div>

            <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={onCreateAnother}>
                    Create another
                </Button>
                <Button onClick={onDone}>Done</Button>
            </div>
        </div>
    )
}
