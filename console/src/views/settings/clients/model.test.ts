// Unit tests for the clients access model's pure logic: constraint pruning,
// restrictable-grant derivation, and the AuthMethod <-> Client adapters
// (including verifyWith pem/jwks branching and role/grants mutual exclusion).

import { describe, expect, it } from "vitest"

import {
    activeConstraints,
    authMethodToClient,
    clientToCreateParams,
    clientToUpdateParams,
    restrictableGrants,
    type Client,
} from "./model"
import type { PermissionSelection } from "../PermissionSelector"
import type { AuthMethod, PermissionGrant } from "@/types"

// A custom permission selection that grants :create on the given resources, so
// tests can drive restrictableGrants/activeConstraints precisely without relying
// on a role preset's exact expansion.
function customCreate(resources: string[]): PermissionSelection {
    return {
        kind: "custom",
        grants: resources.map((resource): PermissionGrant => ({ resource, verb: "create" })),
    }
}

function clientWith(overrides: Partial<Client>): Client {
    return {
        id: "",
        name: "Test",
        identity: { type: "api_key" },
        permissions: { kind: "role", role: "support" },
        subjectScope: "all",
        constraints: {},
        ...overrides,
    }
}

describe("restrictableGrants", () => {
    it("returns only restrictable resources that have a :create grant", () => {
        const client = clientWith({
            permissions: customCreate(["events", "subscriptions"]),
        })
        expect(restrictableGrants(client)).toEqual(["events", "subscriptions"])
    })

    it("preserves the canonical resource order (events, subscriptions, scheduled)", () => {
        const client = clientWith({
            // Provide in a different order than the canonical list.
            permissions: customCreate(["scheduled", "subscriptions", "events"]),
        })
        expect(restrictableGrants(client)).toEqual(["events", "subscriptions", "scheduled"])
    })

    it("ignores non-create verbs on restrictable resources", () => {
        const client = clientWith({
            permissions: {
                kind: "custom",
                grants: [
                    { resource: "events", verb: "read" },
                    { resource: "events", verb: "update" },
                ],
            },
        })
        expect(restrictableGrants(client)).toEqual([])
    })

    it("ignores create grants on non-restrictable resources", () => {
        const client = clientWith({
            permissions: customCreate(["users", "campaigns"]),
        })
        expect(restrictableGrants(client)).toEqual([])
    })

    it("derives create grants from a role preset (editor restricts all three)", () => {
        const client = clientWith({ permissions: { kind: "role", role: "editor" } })
        expect(restrictableGrants(client)).toEqual(["events", "subscriptions", "scheduled"])
    })

    it("derives create grants from a role preset (client lacks subscriptions:create)", () => {
        // subscriptions:create requires the editor role; client does not satisfy it.
        const client = clientWith({ permissions: { kind: "role", role: "client" } })
        expect(restrictableGrants(client)).toEqual(["events", "scheduled"])
    })

    it("returns nothing for a read-only role preset (support)", () => {
        const client = clientWith({ permissions: { kind: "role", role: "support" } })
        expect(restrictableGrants(client)).toEqual([])
    })
})

describe("activeConstraints", () => {
    it("keeps a non-empty list for a resource that is in scope", () => {
        const client = clientWith({
            permissions: customCreate(["events"]),
            constraints: { events: ["signup", "login"] },
        })
        expect(activeConstraints(client)).toEqual({ events: ["signup", "login"] })
    })

    it("drops an empty list so an allow-nothing constraint is never persisted", () => {
        const client = clientWith({
            permissions: customCreate(["events"]),
            constraints: { events: [] },
        })
        expect(activeConstraints(client)).toEqual({})
    })

    it("drops constraints for resources that have fallen out of create scope", () => {
        // Typed names are kept on the client model, but pruned on save because the
        // resource no longer has a :create grant.
        const client = clientWith({
            permissions: customCreate(["events"]),
            constraints: { events: ["a"], scheduled: ["b"] },
        })
        expect(activeConstraints(client)).toEqual({ events: ["a"] })
    })

    it("prunes every out-of-scope resource when no create grants remain", () => {
        const client = clientWith({
            permissions: { kind: "role", role: "support" },
            constraints: { events: ["a"], subscriptions: ["b"], scheduled: ["c"] },
        })
        expect(activeConstraints(client)).toEqual({})
    })

    it("never includes non-restrictable resources even if present in constraints", () => {
        const client = clientWith({
            permissions: customCreate(["events", "users"]),
            // "users" is not a restrictable resource; an entry for it must be ignored.
            constraints: { events: ["a"], users: ["b"] } as Client["constraints"],
        })
        expect(activeConstraints(client)).toEqual({ events: ["a"] })
    })

    it("returns a fresh object and does not mutate the client's constraints", () => {
        const constraints = { events: ["a"] }
        const client = clientWith({
            permissions: customCreate(["events"]),
            constraints,
        })
        const out = activeConstraints(client)
        expect(out).not.toBe(constraints)
        expect(client.constraints).toEqual({ events: ["a"] })
    })
})

describe("authMethodToClient", () => {
    const base = {
        id: "am_1",
        project_id: "proj_1",
        name: "My key",
        role: "support" as const,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
    }

    it("maps an api_key method to an api_key identity with a role selection", () => {
        const m: AuthMethod = { ...base, type: "api_key", description: "desc" }
        const c = authMethodToClient(m)
        expect(c.id).toBe("am_1")
        expect(c.name).toBe("My key")
        expect(c.description).toBe("desc")
        expect(c.identity).toEqual({ type: "api_key" })
        expect(c.permissions).toEqual({ kind: "role", role: "support" })
        expect(c.subjectScope).toBe("all")
        expect(c.constraints).toEqual({})
    })

    it("prefers explicit grants (custom) over the role when grants are present", () => {
        const grants: PermissionGrant[] = [{ resource: "events", verb: "create" }]
        const m: AuthMethod = { ...base, type: "api_key", grants }
        expect(authMethodToClient(m).permissions).toEqual({ kind: "custom", grants })
    })

    it("falls back to the role selection when grants is an empty array", () => {
        const m: AuthMethod = { ...base, type: "api_key", role: "editor", grants: [] }
        expect(authMethodToClient(m).permissions).toEqual({ kind: "role", role: "editor" })
    })

    it("defaults subject_scope to 'all' and grant_constraints to {} when absent", () => {
        const m: AuthMethod = { ...base, type: "api_key" }
        const c = authMethodToClient(m)
        expect(c.subjectScope).toBe("all")
        expect(c.constraints).toEqual({})
    })

    it("carries through 'own' subject scope and grant constraints", () => {
        const m: AuthMethod = {
            ...base,
            type: "api_key",
            subject_scope: "own",
            grant_constraints: { events: ["signup"] },
        }
        const c = authMethodToClient(m)
        expect(c.subjectScope).toBe("own")
        expect(c.constraints).toEqual({ events: ["signup"] })
    })

    it("maps a trusted_issuer with a public_cert to the pem verify mode", () => {
        const m: AuthMethod = {
            ...base,
            type: "trusted_issuer",
            trusted_issuer: {
                public_cert: "-----BEGIN PUBLIC KEY-----",
                iss: "https://issuer.example",
                aud: "my-aud",
                claim: { sub: "user_id" },
            },
        }
        expect(authMethodToClient(m).identity).toEqual({
            type: "trusted_issuer",
            verifyWith: "pem",
            jwksUrl: undefined,
            publicCert: "-----BEGIN PUBLIC KEY-----",
            iss: "https://issuer.example",
            aud: "my-aud",
            subjectClaim: "user_id",
        })
    })

    it("maps a trusted_issuer without a cert to the jwks verify mode and defaults the sub claim", () => {
        const m: AuthMethod = {
            ...base,
            type: "trusted_issuer",
            trusted_issuer: { jwks_url: "https://issuer.example/jwks", iss: "https://issuer.example" },
        }
        expect(authMethodToClient(m).identity).toEqual({
            type: "trusted_issuer",
            verifyWith: "jwks",
            jwksUrl: "https://issuer.example/jwks",
            publicCert: undefined,
            iss: "https://issuer.example",
            aud: undefined,
            subjectClaim: "sub",
        })
    })

    it("defaults a trusted_issuer with no config block to jwks with a 'sub' claim", () => {
        const m: AuthMethod = { ...base, type: "trusted_issuer" }
        expect(authMethodToClient(m).identity).toEqual({
            type: "trusted_issuer",
            verifyWith: "jwks",
            jwksUrl: undefined,
            publicCert: undefined,
            iss: undefined,
            aud: undefined,
            subjectClaim: "sub",
        })
    })

    it("maps a session method, defaulting ttl to 900 when absent", () => {
        const m: AuthMethod = { ...base, type: "session" }
        expect(authMethodToClient(m).identity).toEqual({ type: "session", ttlSeconds: 900 })
    })

    it("carries through a session ttl_seconds", () => {
        const m: AuthMethod = { ...base, type: "session", session: { ttl_seconds: 60 } }
        expect(authMethodToClient(m).identity).toEqual({ type: "session", ttlSeconds: 60 })
    })
})

describe("clientToCreateParams", () => {
    it("trims name/description and sends a role with an empty grants list for a role preset", () => {
        const params = clientToCreateParams(
            clientWith({
                name: "  My key  ",
                description: "  desc  ",
                permissions: { kind: "role", role: "editor" },
            }),
        )
        expect(params.name).toBe("My key")
        expect(params.description).toBe("desc")
        expect(params.role).toBe("editor")
        // Role and grants are mutually exclusive effective scopes: a preset clears
        // any custom grants.
        expect(params.grants).toEqual([])
    })

    it("omits an empty/whitespace description", () => {
        const params = clientToCreateParams(clientWith({ description: "   " }))
        expect(params.description).toBeUndefined()
    })

    it("sends explicit grants and no role for a custom selection", () => {
        const grants: PermissionGrant[] = [{ resource: "events", verb: "create" }]
        const params = clientToCreateParams(
            clientWith({ permissions: { kind: "custom", grants } }),
        )
        expect(params.grants).toEqual(grants)
        expect(params.role).toBeUndefined()
    })

    it("includes only pruned (active) grant constraints", () => {
        const params = clientToCreateParams(
            clientWith({
                permissions: customCreate(["events"]),
                constraints: { events: ["signup"], scheduled: ["out-of-scope"] },
            }),
        )
        expect(params.grant_constraints).toEqual({ events: ["signup"] })
    })

    it("emits no identity config block for an api_key", () => {
        const params = clientToCreateParams(clientWith({ identity: { type: "api_key" } }))
        expect(params.type).toBe("api_key")
        expect(params.trusted_issuer).toBeUndefined()
        expect(params.session).toBeUndefined()
    })

    it("for a trusted_issuer in jwks mode, sends jwks_url and clears public_cert", () => {
        const params = clientToCreateParams(
            clientWith({
                identity: {
                    type: "trusted_issuer",
                    verifyWith: "jwks",
                    jwksUrl: "https://issuer.example/jwks",
                    publicCert: "should-be-ignored",
                    iss: "https://issuer.example",
                    aud: "my-aud",
                    subjectClaim: "user_id",
                },
            }),
        )
        expect(params.trusted_issuer).toEqual({
            jwks_url: "https://issuer.example/jwks",
            public_cert: undefined,
            iss: "https://issuer.example",
            aud: "my-aud",
            claim: { sub: "user_id" },
        })
    })

    it("for a trusted_issuer in pem mode, sends public_cert and clears jwks_url (XOR)", () => {
        const params = clientToCreateParams(
            clientWith({
                identity: {
                    type: "trusted_issuer",
                    verifyWith: "pem",
                    jwksUrl: "should-be-ignored",
                    publicCert: "-----BEGIN PUBLIC KEY-----",
                    iss: "https://issuer.example",
                },
            }),
        )
        expect(params.trusted_issuer?.public_cert).toBe("-----BEGIN PUBLIC KEY-----")
        expect(params.trusted_issuer?.jwks_url).toBeUndefined()
    })

    it("defaults the trusted_issuer sub claim to 'sub' and omits an empty aud", () => {
        const params = clientToCreateParams(
            clientWith({
                identity: {
                    type: "trusted_issuer",
                    verifyWith: "jwks",
                    jwksUrl: "https://issuer.example/jwks",
                    iss: "https://issuer.example",
                    aud: "",
                    subjectClaim: "",
                },
            }),
        )
        expect(params.trusted_issuer?.claim).toEqual({ sub: "sub" })
        expect(params.trusted_issuer?.aud).toBeUndefined()
    })

    it("sends the session ttl config block", () => {
        const params = clientToCreateParams(
            clientWith({ identity: { type: "session", ttlSeconds: 300 } }),
        )
        expect(params.session).toEqual({ ttl_seconds: 300 })
        expect(params.type).toBe("session")
    })
})

describe("clientToUpdateParams", () => {
    it("trims name and sends an empty-string description rather than undefined", () => {
        const params = clientToUpdateParams(
            clientWith({ name: "  Renamed  ", description: "   " }),
        )
        expect(params.name).toBe("Renamed")
        expect(params.description).toBe("")
    })

    it("applies role/grants mutual exclusion for a role preset", () => {
        const params = clientToUpdateParams(
            clientWith({ permissions: { kind: "role", role: "admin" } }),
        )
        expect(params.role).toBe("admin")
        expect(params.grants).toEqual([])
    })

    it("sends explicit grants and no role for a custom selection", () => {
        const grants: PermissionGrant[] = [{ resource: "events", verb: "create" }]
        const params = clientToUpdateParams(
            clientWith({ permissions: { kind: "custom", grants } }),
        )
        expect(params.grants).toEqual(grants)
        expect(params.role).toBeUndefined()
    })

    it("includes only pruned (active) grant constraints", () => {
        const params = clientToUpdateParams(
            clientWith({
                permissions: customCreate(["events"]),
                constraints: { events: ["signup"], scheduled: ["gone"] },
            }),
        )
        expect(params.grant_constraints).toEqual({ events: ["signup"] })
    })

    it("does not carry identity authentication config (update is identity-immutable)", () => {
        const params = clientToUpdateParams(
            clientWith({ identity: { type: "session", ttlSeconds: 300 } }),
        )
        expect(params).not.toHaveProperty("session")
        expect(params).not.toHaveProperty("trusted_issuer")
        expect(params).not.toHaveProperty("type")
    })
})

describe("round-trip: authMethodToClient -> clientToCreateParams", () => {
    const base = {
        id: "am_1",
        project_id: "proj_1",
        name: "My key",
        role: "support" as const,
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
    }

    it("preserves a trusted_issuer pem configuration through the round trip", () => {
        const m: AuthMethod = {
            ...base,
            type: "trusted_issuer",
            subject_scope: "own",
            grants: [{ resource: "events", verb: "create" }],
            grant_constraints: { events: ["signup"] },
            trusted_issuer: {
                public_cert: "-----BEGIN PUBLIC KEY-----",
                iss: "https://issuer.example",
                aud: "my-aud",
                claim: { sub: "user_id" },
            },
        }
        const params = clientToCreateParams(authMethodToClient(m))
        expect(params.type).toBe("trusted_issuer")
        expect(params.subject_scope).toBe("own")
        expect(params.grants).toEqual([{ resource: "events", verb: "create" }])
        expect(params.grant_constraints).toEqual({ events: ["signup"] })
        expect(params.trusted_issuer).toEqual({
            jwks_url: undefined,
            public_cert: "-----BEGIN PUBLIC KEY-----",
            iss: "https://issuer.example",
            aud: "my-aud",
            claim: { sub: "user_id" },
        })
    })

    it("preserves a trusted_issuer jwks configuration through the round trip", () => {
        const m: AuthMethod = {
            ...base,
            type: "trusted_issuer",
            trusted_issuer: {
                jwks_url: "https://issuer.example/jwks",
                iss: "https://issuer.example",
                claim: { sub: "sub" },
            },
        }
        const params = clientToCreateParams(authMethodToClient(m))
        expect(params.trusted_issuer?.jwks_url).toBe("https://issuer.example/jwks")
        expect(params.trusted_issuer?.public_cert).toBeUndefined()
    })
})
