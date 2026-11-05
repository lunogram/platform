/**
 * ApiError is a failed openapi-fetch request turned into a thrown error.
 *
 * The axios client throws on 4xx/5xx, but openapi-fetch returns the error
 * instead, so code that only reads `data` treats a rejected request as a
 * success. `unwrap` gives openapi-fetch calls the axios behaviour, which is
 * what TanStack Query needs to see a request as failed.
 */
export class ApiError extends Error {
    /** The response body openapi-fetch returned as the error. */
    readonly problem: unknown
    /** The RFC 7807 "detail" of the response, when the server supplied one. */
    readonly detail: string | null

    constructor(problem: unknown) {
        const detail =
            typeof problem === "object" &&
            problem !== null &&
            "detail" in problem &&
            typeof problem.detail === "string"
                ? problem.detail
                : null
        super(detail ?? "Request failed")
        this.name = "ApiError"
        this.problem = problem
        this.detail = detail
    }
}

/**
 * @returns the data of an openapi-fetch result, throwing an ApiError when the
 * request failed.
 */
export function unwrap<T>(result: { data?: T; error?: unknown }): T {
    if (result.error !== undefined) {
        throw new ApiError(result.error)
    }
    return result.data as T
}
