import { QueryClient } from "@tanstack/react-query"

/**
 * The shared QueryClient for the console.
 *
 * Defaults are tuned for an authenticated dashboard where most data is
 * project-scoped and changes from other tabs/sessions are rare:
 *
 * - `staleTime` of 30s keeps navigation snappy (cached data is shown
 *   instantly) while still revalidating in the background often enough that
 *   the UI never feels stale.
 * - `retry: 1` avoids hammering the API on genuine 4xx errors while still
 *   tolerating a single transient blip.
 * - `refetchOnWindowFocus` is left on (the default) so returning to a tab
 *   pulls fresh data without a manual reload.
 */
export const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            staleTime: 30_000,
            retry: 1,
        },
    },
})
