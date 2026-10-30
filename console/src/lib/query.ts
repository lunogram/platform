import { MutationCache, QueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { toast } from "sonner"
import { problemDetail } from "@/utils"

declare module "@tanstack/react-query" {
    interface Register {
        mutationMeta: {
            /** Shown when the mutation fails and the server gave no detail. */
            errorMessage?: string
        }
    }
}

// Errors the mutation cache already showed, so callers that also catch the
// error (e.g. to stay on a page) don't toast it a second time.
const reportedErrors = new WeakSet<object>()

/** @returns whether a failed mutation already told the user about this error. */
export function wasReported(err: unknown): boolean {
    return typeof err === "object" && err !== null && reportedErrors.has(err)
}

export const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            // Long enough that moving between the steps of one editor reuses the
            // cache instead of refetching; short enough that returning later is fresh.
            staleTime: 30_000,
            // The default of three retries delays an error by several seconds.
            retry: 1,
        },
    },
    mutationCache: new MutationCache({
        // Every failed save is shown, so no call site can fail silently.
        onError: (err, _variables, _onMutateResult, mutation) => {
            if (typeof err === "object" && err !== null) reportedErrors.add(err)
            toast.error(
                problemDetail(err) ??
                    mutation.meta?.errorMessage ??
                    t("request_failed", "Something went wrong. Please try again."),
            )
        },
    }),
})
