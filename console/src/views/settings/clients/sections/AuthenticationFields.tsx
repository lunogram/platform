import { Clock, KeyRound, ShieldCheck } from "lucide-react"

import {
    describeIdentity,
    hasVerifiedSubject,
    identityMeta,
    newIdentity,
    type Client,
    type Identity,
    type IdentityType,
} from "../model"
import { Field } from "../form-parts"
import { RequestPreview } from "../RequestPreview"

import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { SelectableCard } from "@/components/ui/selectable-card"

const identityIcon: Record<IdentityType, typeof KeyRound> = {
    api_key: KeyRound,
    trusted_issuer: ShieldCheck,
    session: Clock,
}

// AuthenticationFields: the method a client proves itself with, its config, the
// data boundary that method allows, and a live example of authenticating with it.
export function AuthenticationFields({
    client,
    set,
    updateIdentity,
    verified,
    readOnly = false,
}: {
    client: Client
    set: (patch: Partial<Client>) => void
    updateIdentity: (patch: Partial<Identity>) => void
    verified: boolean
    // readOnly fixes the authentication method and its config (used when editing
    // an existing client — the API does not allow changing how it authenticates).
    // The data-access boundary below stays editable.
    readOnly?: boolean
}) {
    const setIdentityType = (type: IdentityType) => {
        const identity = newIdentity(type)
        const patch = { identity } as { identity: Identity; subjectScope?: "all" | "own" }
        if (!hasVerifiedSubject(identity) && client.subjectScope === "own")
            patch.subjectScope = "all"
        set(patch)
    }

    return (
        <div className="grid gap-8">
            {readOnly ? (
                <ReadOnlyIdentity identity={client.identity} />
            ) : (
                <>
                    <div className="grid gap-3">
                        <SubLabel>Method</SubLabel>
                        <div
                            role="group"
                            aria-label="Authentication method"
                            className="grid gap-2 sm:grid-cols-3"
                        >
                            {(Object.keys(identityMeta) as IdentityType[]).map((type) => {
                                const Icon = identityIcon[type]
                                return (
                                    <SelectableCard
                                        key={type}
                                        active={client.identity.type === type}
                                        onClick={() => setIdentityType(type)}
                                        icon={<Icon className="h-4 w-4" />}
                                        title={identityMeta[type].label}
                                        summary={identityMeta[type].blurb}
                                    />
                                )
                            })}
                        </div>
                    </div>

                    <IdentityConfig identity={client.identity} onChange={updateIdentity} />
                </>
            )}

            {verified && (
                <div className="grid gap-3">
                    <SubLabel hint="Whose data this client may touch within its permissions.">
                        Data access
                    </SubLabel>
                    <div
                        role="group"
                        aria-label="Data access"
                        className="grid gap-2 sm:grid-cols-2"
                    >
                        <SelectableCard
                            active={client.subjectScope === "all"}
                            onClick={() => set({ subjectScope: "all" })}
                            title="All data"
                            summary="Every user's records."
                        />
                        <SelectableCard
                            active={client.subjectScope === "own"}
                            onClick={() => set({ subjectScope: "own" })}
                            title="Own data only"
                            summary="The authenticated user's records."
                        />
                    </div>
                </div>
            )}

            <div className="grid gap-3">
                <SubLabel hint={exampleHint(client.identity.type)}>Example</SubLabel>
                <RequestPreview client={client} />
            </div>
        </div>
    )
}

function exampleHint(type: IdentityType): string {
    if (type === "api_key")
        return "A secret key is generated on creation — send it as a Bearer token."
    if (type === "trusted_issuer")
        return "Your identity provider issues tokens; Lunogram verifies them."
    return "Your backend mints short-lived tokens for this client by calling the sessions endpoint with a secret API key."
}

// ReadOnlyIdentity shows how an existing client authenticates without letting it
// be changed — authentication is fixed once a client is created.
function ReadOnlyIdentity({ identity }: { identity: Identity }) {
    const Icon = identityIcon[identity.type]
    return (
        <div className="grid gap-3">
            <SubLabel hint="Authentication is fixed after creation. To change it, create a new client.">
                Method
            </SubLabel>
            <div className="flex items-center gap-3 rounded-md border bg-surface-soft px-3.5 py-3">
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-soft">
                    <Icon className="h-[18px] w-[18px]" strokeWidth={1.5} />
                </span>
                <div className="grid min-w-0 gap-0.5">
                    <span className="text-sm font-medium">{identityMeta[identity.type].label}</span>
                    <span className="truncate font-mono text-xs text-ink-soft">
                        {describeIdentity(identity)}
                    </span>
                </div>
            </div>
        </div>
    )
}

function SubLabel({ children, hint }: { children: React.ReactNode; hint?: string }) {
    return (
        <div className="grid gap-0.5">
            <span className="text-sm font-medium">{children}</span>
            {hint && <span className="text-xs text-ink-soft">{hint}</span>}
        </div>
    )
}

function IdentityConfig({
    identity,
    onChange,
}: {
    identity: Identity
    onChange: (patch: Partial<Identity>) => void
}) {
    // The API key has no configuration — the key itself is generated on creation.
    if (identity.type === "api_key") return null

    if (identity.type === "session") {
        return (
            <div className="grid gap-x-6 gap-y-5 sm:grid-cols-2">
                <Field label="Token lifetime (seconds)" htmlFor="cl-ttl">
                    <Input
                        id="cl-ttl"
                        type="number"
                        min={1}
                        value={identity.ttlSeconds ?? 900}
                        onChange={(e) => {
                            const n = Number(e.target.value)
                            onChange({ ttlSeconds: Number.isFinite(n) && n > 0 ? n : undefined })
                        }}
                    />
                </Field>
            </div>
        )
    }

    // trusted_issuer
    const verifyWith = identity.verifyWith ?? (identity.publicCert ? "pem" : "jwks")
    return (
        <div className="grid gap-5">
            <div className="grid gap-3">
                <SubLabel>Verify tokens with</SubLabel>
                <div className="grid gap-2 sm:grid-cols-2">
                    <SelectableCard
                        active={verifyWith === "jwks"}
                        onClick={() => onChange({ verifyWith: "jwks" })}
                        title="JWKS URL"
                        summary="Fetch keys from your identity provider."
                    />
                    <SelectableCard
                        active={verifyWith === "pem"}
                        onClick={() => onChange({ verifyWith: "pem" })}
                        title="Public key"
                        summary="Paste a PEM key or cert."
                    />
                </div>
            </div>

            <div className="grid gap-x-6 gap-y-5 sm:grid-cols-2">
                {verifyWith === "pem" ? (
                    <div className="sm:col-span-2">
                        <Field
                            label="Public key (PEM)"
                            hint="Your identity provider's PEM-encoded public key or X.509 certificate. Signatures are verified against it directly — no network fetch."
                            htmlFor="cl-public-cert"
                        >
                            <Textarea
                                id="cl-public-cert"
                                rows={5}
                                className="font-mono text-xs"
                                placeholder={"-----BEGIN PUBLIC KEY-----\n…"}
                                value={identity.publicCert ?? ""}
                                onChange={(e) => onChange({ publicCert: e.target.value })}
                            />
                        </Field>
                    </div>
                ) : (
                    <Field
                        label="JWKS URL"
                        hint="Where we fetch your identity provider's public signing keys."
                        htmlFor="cl-jwks-url"
                    >
                        <Input
                            id="cl-jwks-url"
                            value={identity.jwksUrl ?? ""}
                            placeholder="https://issuer/.well-known/jwks.json"
                            onChange={(e) => onChange({ jwksUrl: e.target.value })}
                        />
                    </Field>
                )}
                <Field
                    label="Issuer (iss)"
                    hint="Must match the iss claim in incoming tokens."
                    htmlFor="cl-iss"
                >
                    <Input
                        id="cl-iss"
                        value={identity.iss ?? ""}
                        placeholder="https://auth.acme.com/"
                        onChange={(e) => onChange({ iss: e.target.value })}
                    />
                </Field>
                <Field
                    label="Audience (aud)"
                    hint="Optional. Leave empty to skip the check."
                    htmlFor="cl-aud"
                >
                    <Input
                        id="cl-aud"
                        value={identity.aud ?? ""}
                        onChange={(e) => onChange({ aud: e.target.value })}
                    />
                </Field>
                <Field
                    label="Subject claim"
                    hint="Which token claim holds the user id."
                    htmlFor="cl-subject-claim"
                >
                    <Input
                        id="cl-subject-claim"
                        value={identity.subjectClaim ?? "sub"}
                        onChange={(e) => onChange({ subjectClaim: e.target.value })}
                    />
                </Field>
            </div>
        </div>
    )
}
