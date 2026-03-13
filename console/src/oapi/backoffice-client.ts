import createClient from "openapi-fetch"
import type { paths, components } from "./backoffice.generated"

// Create the openapi-fetch client for the backoffice service.
// Requests are proxied via vite dev server (see vite.config.ts).
export const backofficeClient = createClient<paths>({
    baseUrl: "/backoffice",
    credentials: "include",
})

// Add response interceptor for 401 handling
backofficeClient.use({
    async onResponse({ response }) {
        const isLoginPage = window.location.pathname.startsWith("/login")
        if (response.status === 401 && !isLoginPage) {
            window.location.href = `/login?r=${encodeURIComponent(window.location.href)}`
        }
        return response
    },
})

// Export schema types for convenience
export type Conversation = components["schemas"]["Conversation"]
export type ConversationList = components["schemas"]["ConversationList"]
export type ConversationDetail = components["schemas"]["ConversationDetail"]
export type CreateConversationRequest = components["schemas"]["CreateConversationRequest"]
export type SendMessageRequest = components["schemas"]["SendMessageRequest"]
export type MessageContext = components["schemas"]["MessageContext"]
export type ImageReference = components["schemas"]["ImageReference"]
export type SectionReference = components["schemas"]["SectionReference"]
export type VariableGroup = components["schemas"]["VariableGroup"]
export type Variable = components["schemas"]["Variable"]
export type Message = components["schemas"]["Message"]
export type MessageList = components["schemas"]["MessageList"]
export type TemplateVersion = components["schemas"]["TemplateVersion"]
export type VersionList = components["schemas"]["VersionList"]
export type AgentResponse = components["schemas"]["AgentResponse"]
