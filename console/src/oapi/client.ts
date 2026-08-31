import createClient from "openapi-fetch"
import { env } from "@/config/env"
import { reportCrossOriginRefusal } from "@/lib/cross-origin"
import { isPublicPage } from "@/lib/public-paths"
import type { paths, components } from "./management.generated"

const apiBaseUrl = env.api.baseURL.replace(/\/$/, "")
const oapiBaseUrl = apiBaseUrl.endsWith("/api") ? apiBaseUrl.slice(0, -4) : apiBaseUrl

// Create the openapi-fetch client
export const oapiClient = createClient<paths>({
    baseUrl: oapiBaseUrl,
    credentials: "include",
})

// Add response interceptor for 401 handling
oapiClient.use({
    async onResponse({ response }) {
        if (response.status === 401 && !isPublicPage()) {
            window.location.href = `/login?r=${encodeURIComponent(window.location.href)}`
        }
        if (response.status === 403) {
            reportCrossOriginRefusal(response.status, await response.clone().json().catch(() => null))
        }
        return response
    },
})

// Export schema types for convenience
export type Organization = components["schemas"]["Organization"]
export type OrganizationList = components["schemas"]["OrganizationList"]
export type UpsertOrganization = components["schemas"]["UpsertOrganization"]
export type UpdateOrganization = components["schemas"]["UpdateOrganization"]
export type OrganizationMember = components["schemas"]["OrganizationMember"]
export type OrganizationMemberList = components["schemas"]["OrganizationMemberList"]
export type AddOrganizationMember = components["schemas"]["AddOrganizationMember"]
export type User = components["schemas"]["User"]
export type UserList = components["schemas"]["UserList"]

export type Action = components["schemas"]["Action"]
export type CreateAction = components["schemas"]["CreateAction"]
export type UpdateAction = components["schemas"]["UpdateAction"]
export type ActionMeta = components["schemas"]["ActionMeta"]
export type ActionFunction = components["schemas"]["ActionFunction"]
export type TestActionRequest = components["schemas"]["TestActionRequest"]
export type TestActionResult = components["schemas"]["TestActionResult"]
export type TestActionFunctionRequest = components["schemas"]["TestActionFunctionRequest"]
export type TestActionFunctionResult = components["schemas"]["TestActionFunctionResult"]

export type SenderIdentity = components["schemas"]["SenderIdentity"]
export type CreateSenderIdentity = components["schemas"]["CreateSenderIdentity"]

export type Provider = components["schemas"]["Provider"]
export type CreateProvider = components["schemas"]["CreateProvider"]
export type UpdateProvider = components["schemas"]["UpdateProvider"]
export type ProviderMeta = components["schemas"]["ProviderMeta"]

export default oapiClient
