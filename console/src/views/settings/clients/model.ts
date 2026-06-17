// View model for the API & Clients interface, and the adapters that map it to
// and from the management API's AuthMethod resource.
//
// A "client" is the console's name for an auth method: how an integration
// authenticates (identity), what it may do (permissions), and whose data it may
// touch (subjectScope). The editor works against this shape; the adapters below
// translate it for api.authMethods.

import type { PermissionSelection } from "../PermissionSelector"
import { presetGrants } from "@/lib/rbac-presets"
import type {
    AuthMethod,
    CreateAuthMethodParams,
    PermissionGrant,
    ProjectRole,
    SubjectScope,
    UpdateAuthMethodParams,
} from "@/types"

export type IdentityType = "api_key" | "trusted_issuer" | "session"

// Identity is how a client authenticates. A client has exactly one.
export interface Identity {
    type: IdentityType
    // api_key — always a private (backend-only) sk_ key
    secretPrefix?: string
    // trusted_issuer — verify token signatures either against keys fetched from
    // a JWKS URL, or against a public key/cert pasted directly.
    verifyWith?: "jwks" | "pem"
    jwksUrl?: string
    publicCert?: string
    iss?: string
    aud?: string
    subjectClaim?: string
    // session — minted short-lived tokens; only the lifetime is configured here.
    ttlSeconds?: number
}

export type { SubjectScope }

// Client is a named integration that talks to the API: how it authenticates
// (identity), what it may do (permissions), and whose data it may touch
// (subjectScope). It mirrors a management AuthMethod.
export interface Client {
    // id is empty for an unsaved draft; the server assigns it on creation.
    id: string
    name: string
    description?: string
    identity: Identity
    permissions: PermissionSelection
    subjectScope: SubjectScope
}

export const identityMeta: Record<IdentityType, { label: string; blurb: string }> = {
    api_key: { label: "API key", blurb: "A secret key your backend sends as a Bearer token." },
    trusted_issuer: {
        label: "Trusted issuer",
        blurb: "Verify tokens from your own identity provider.",
    },
    session: { label: "Session", blurb: "Mint short-lived, user-scoped tokens from your backend." },
}

// hasVerifiedSubject reports whether an identity carries a verified end-user
// subject — the precondition for the "own data only" boundary.
export function hasVerifiedSubject(identity: Identity): boolean {
    return identity.type === "trusted_issuer" || identity.type === "session"
}

// describeIdentity renders the machine-owned, copyable detail for an identity.
export function describeIdentity(identity: Identity): string {
    switch (identity.type) {
        case "api_key":
            return identity.secretPrefix ?? "sk_…"
        case "trusted_issuer":
            return identity.iss ?? identity.jwksUrl ?? (identity.publicCert ? "public key" : "—")
        case "session":
            return identity.ttlSeconds ? `${identity.ttlSeconds}s TTL` : "—"
    }
}

// permissionSummary is the short label for a client's permission set.
export function permissionSummary(p: PermissionSelection): string {
    if (p.kind === "role") return p.role.charAt(0).toUpperCase() + p.role.slice(1)
    const n = p.grants.length
    return `Custom · ${n} permission${n === 1 ? "" : "s"}`
}

// grantsFor returns the explicit grants a permission selection confers, whether
// it is a role preset or a custom set.
export function grantsFor(p: PermissionSelection): PermissionGrant[] {
    return p.kind === "role" ? presetGrants(p.role) : p.grants
}

export function newIdentity(type: IdentityType): Identity {
    switch (type) {
        case "api_key":
            return { type }
        case "trusted_issuer":
            return { type, subjectClaim: "sub", verifyWith: "jwks" }
        case "session":
            return { type, ttlSeconds: 900 }
    }
}

// newClient is a blank draft. Verified-subject identities default to "own" data;
// since the default identity is an api_key, the draft starts as "all".
export function newClient(): Client {
    return {
        id: "",
        name: "",
        identity: newIdentity("api_key"),
        permissions: { kind: "role", role: "support" },
        subjectScope: "all",
    }
}

// authMethodToClient maps a stored AuthMethod into the editor's view model.
export function authMethodToClient(m: AuthMethod): Client {
    return {
        id: m.id,
        name: m.name,
        description: m.description,
        identity: identityOf(m),
        permissions:
            m.grants && m.grants.length > 0
                ? { kind: "custom", grants: m.grants }
                : { kind: "role", role: m.role },
        subjectScope: m.subject_scope ?? "all",
    }
}

function identityOf(m: AuthMethod): Identity {
    switch (m.type) {
        case "trusted_issuer": {
            const ti = m.trusted_issuer ?? {}
            return {
                type: "trusted_issuer",
                verifyWith: ti.public_cert ? "pem" : "jwks",
                jwksUrl: ti.jwks_url,
                publicCert: ti.public_cert,
                iss: ti.iss,
                aud: ti.aud,
                subjectClaim: ti.claim?.sub ?? "sub",
            }
        }
        case "session":
            return { type: "session", ttlSeconds: m.session?.ttl_seconds ?? 900 }
        default:
            return { type: "api_key" }
    }
}

// permissionParams splits a permission selection into the role/grants fields the
// API expects: a preset sends a role, a custom set sends explicit grants.
function permissionParams(p: PermissionSelection): {
    role?: ProjectRole
    grants?: PermissionGrant[]
} {
    return p.kind === "role" ? { role: p.role } : { grants: p.grants }
}

// clientToCreateParams maps a draft to a create request. Only the identity's own
// configuration block is included.
export function clientToCreateParams(c: Client): CreateAuthMethodParams {
    const params: CreateAuthMethodParams = {
        type: c.identity.type,
        name: c.name.trim(),
        description: c.description?.trim() || undefined,
        subject_scope: c.subjectScope,
        ...permissionParams(c.permissions),
    }
    switch (c.identity.type) {
        case "api_key":
            // No extra configuration — api keys are private sk_ keys.
            break
        case "trusted_issuer":
            params.trusted_issuer = {
                jwks_url: c.identity.verifyWith === "pem" ? undefined : c.identity.jwksUrl,
                public_cert: c.identity.verifyWith === "pem" ? c.identity.publicCert : undefined,
                iss: c.identity.iss,
                aud: c.identity.aud || undefined,
                claim: { sub: c.identity.subjectClaim || "sub" },
            }
            break
        case "session":
            params.session = { ttl_seconds: c.identity.ttlSeconds }
            break
    }
    return params
}

// clientToUpdateParams maps a draft to an update request. The API only mutates
// name, description, permissions and data scope — an identity's authentication
// config is fixed once created.
export function clientToUpdateParams(c: Client): UpdateAuthMethodParams {
    return {
        name: c.name.trim(),
        description: c.description?.trim() || "",
        subject_scope: c.subjectScope,
        ...permissionParams(c.permissions),
    }
}
