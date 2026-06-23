import { useMemo, useState } from "react"
import { Check, Copy, Terminal } from "lucide-react"

import { grantsFor, type Client } from "./model"
import type { GrantVerb } from "@/types"

import { cn } from "@/utils"

// Token colors for the dark preview. Kept literal (this panel is always dark,
// regardless of theme) rather than themed.
const C = {
    dim: "text-white/40",
    base: "text-white/85",
    flag: "text-white/55",
    method: "text-[#7ee2a8]",
    url: "text-[#9ec5ff]",
    token: "text-[#d8b4fe]",
}

type Seg = { t: string; c: string }
type Line = Seg[]

const VERB_METHOD: Record<GrantVerb, string> = {
    read: "GET",
    create: "POST",
    update: "PATCH",
    delete: "DELETE",
}

const API = "https://api.lunogram.com"

// pickEndpoint chooses a representative call for the client's grants — a read if
// it has one (clean GET), otherwise its first grant.
function pickEndpoint(client: Client): { method: string; path: string } {
    const grants = grantsFor(client.permissions)
    if (!grants.length) return { method: "GET", path: "/v1/users" }
    const g = grants.find((x) => x.verb === "read") ?? grants[0]
    return { method: VERB_METHOD[g.verb], path: `/v1/${g.resource}` }
}

function buildExample(client: Client, secret?: string): { lines: Line[]; plain: string } {
    const { method, path } = pickEndpoint(client)
    const lines: Line[] = []
    const own = client.subjectScope === "own"
    const key = secret ?? "sk_live_…"

    const callLines = (token: { t: string; c: string }, indent = false): Line[] => {
        const head: Line = [{ t: indent ? "  curl " : "curl ", c: C.base }]
        if (method !== "GET") head.push({ t: `-X ${method} `, c: C.method })
        head.push({ t: `${API}${path}`, c: C.url }, { t: " \\", c: C.dim })
        return [
            head,
            [{ t: '  -H "Authorization: Bearer ', c: C.flag }, token, { t: '"', c: C.flag }],
        ]
    }

    if (client.identity.type === "api_key") {
        lines.push(...callLines({ t: key, c: C.token }))
    } else if (client.identity.type === "trusted_issuer") {
        lines.push([
            { t: "# Your identity provider issues the token; Lunogram verifies it.", c: C.dim },
        ])
        lines.push(...callLines({ t: "<user token>", c: C.token }))
    } else {
        lines.push([{ t: "# 1 — Your backend mints a short-lived token", c: C.dim }])
        lines.push([
            { t: "curl ", c: C.base },
            { t: "-X POST ", c: C.method },
            { t: `${API}/api/client/sessions`, c: C.url },
            { t: " \\", c: C.dim },
        ])
        lines.push([
            { t: '  -H "Authorization: Bearer ', c: C.flag },
            { t: key, c: C.token },
            { t: '" \\', c: C.flag },
        ])
        lines.push([{ t: `  -d '{"sub": "user_123"}'`, c: C.base }])
        lines.push([])
        lines.push([{ t: "# 2 — Your frontend calls the API with that token", c: C.dim }])
        lines.push(...callLines({ t: "<session token>", c: C.token }))
    }

    if (own) {
        lines.push([])
        lines.push([{ t: "# Requests act on the authenticated user's own data.", c: C.dim }])
    }

    const plain = lines.map((line) => line.map((s) => s.t).join("")).join("\n")
    return { lines, plain }
}

// RequestPreview shows a live, copyable example call that reflects the client's
// current authentication, permissions, and data scope.
export function RequestPreview({ client, secret }: { client: Client; secret?: string }) {
    const { lines, plain } = useMemo(() => buildExample(client, secret), [client, secret])
    const [copied, setCopied] = useState(false)

    const copy = () => {
        void navigator.clipboard?.writeText(plain)
        setCopied(true)
        window.setTimeout(() => setCopied(false), 1500)
    }

    return (
        <div className="max-w-6xl overflow-hidden rounded-xl border border-white/10 bg-surface-editor">
            <div className="flex items-center justify-between border-b border-white/10 px-4 py-2.5">
                <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-white/50">
                    <Terminal className="h-3.5 w-3.5" strokeWidth={1.75} />
                    Example request
                </div>
                <button
                    type="button"
                    onClick={copy}
                    className="flex items-center gap-1.5 rounded-md px-2 py-1 text-xs text-white/70 transition-colors hover:bg-white/10 hover:text-white"
                >
                    {copied ? (
                        <>
                            <Check className="h-3.5 w-3.5" />
                            Copied
                        </>
                    ) : (
                        <>
                            <Copy className="h-3.5 w-3.5" />
                            Copy
                        </>
                    )}
                </button>
            </div>
            <pre className="overflow-x-auto px-4 py-4 font-mono text-[13px] leading-relaxed">
                <code>
                    {lines.map((line, i) => (
                        <div key={i}>
                            {line.length ? (
                                line.map((seg, j) => (
                                    <span key={j} className={cn(seg.c)}>
                                        {seg.t}
                                    </span>
                                ))
                            ) : (
                                <span>{" "}</span>
                            )}
                        </div>
                    ))}
                </code>
            </pre>
        </div>
    )
}
