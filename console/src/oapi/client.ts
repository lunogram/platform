import createClient from "openapi-fetch"
import type { paths, components } from "./management.generated"

// Create the openapi-fetch client
// Note: OpenAPI paths already include /api prefix, so we use empty baseUrl
export const oapiClient = createClient<paths>({
    baseUrl: "",
    credentials: "include",
})

// Add response interceptor for 401 handling
oapiClient.use({
    async onResponse({ response }) {
        const isLoginPage = window.location.pathname.startsWith("/login")
        if (response.status === 401 && !isLoginPage) {
            window.location.href = `/login?r=${encodeURIComponent(window.location.href)}`
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

export default oapiClient
