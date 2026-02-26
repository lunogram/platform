import createClient from "openapi-fetch";
import type { paths, components } from "./management.generated";

// Create the openapi-fetch client
// Note: OpenAPI paths already include /api prefix, so we use empty baseUrl
export const oapiClient = createClient<paths>({
  baseUrl: "",
  credentials: "include",
});

// Add response interceptor for 401 handling
oapiClient.use({
  async onResponse({ response }) {
    const isLoginPage = window.location.pathname.startsWith("/login");
    if (response.status === 401 && !isLoginPage) {
      window.location.href = `/login?r=${encodeURIComponent(window.location.href)}`;
    }
    return response;
  },
});

export type User = components["schemas"]["User"];
export type UserList = components["schemas"]["UserList"];

export default oapiClient;