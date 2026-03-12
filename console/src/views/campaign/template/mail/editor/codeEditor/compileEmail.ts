/**
 * Thin wrapper around the compile Web Worker.
 *
 * The actual compilation (Sucrase transform, `new Function()` eval, React
 * render) runs inside `compileWorker.ts` on a separate thread. This module
 * manages the worker lifecycle and exposes the same `compileEmail()` API so
 * all existing callers work without changes.
 */

export interface CompileResult {
    html: string
    plainText: string
}

interface WorkerRequest {
    id: number
    source: string
    previewProps: Record<string, unknown>
}

interface WorkerSuccess {
    id: number
    html: string
    plainText: string
}

interface WorkerError {
    id: number
    error: string
}

type WorkerResponse = WorkerSuccess | WorkerError

let worker: Worker | null = null
let nextId = 0
const pending = new Map<
    number,
    { resolve: (r: CompileResult) => void; reject: (e: Error) => void }
>()

function getWorker(): Worker {
    if (!worker) {
        worker = new Worker(new URL("./compileWorker.ts", import.meta.url), {
            type: "module",
        })
        worker.onmessage = (e: MessageEvent<WorkerResponse>) => {
            const { id } = e.data
            const entry = pending.get(id)
            if (!entry) return
            pending.delete(id)

            if ("error" in e.data) {
                entry.reject(new Error(e.data.error))
            } else {
                entry.resolve({ html: e.data.html, plainText: e.data.plainText })
            }
        }
        worker.onerror = (e) => {
            // Reject all pending requests on fatal worker error
            const err = new Error(e.message || "Worker error")
            for (const [id, entry] of pending) {
                entry.reject(err)
                pending.delete(id)
            }
        }
    }
    return worker
}

/**
 * Compile a React Email JSX source string into rendered HTML and plain text.
 *
 * The compilation runs in a Web Worker to keep the main thread responsive
 * and to isolate `new Function()` eval in a separate global scope.
 *
 * @param source - The raw JSX/TSX source code from the editor
 * @param previewProps - Props object with placeholder values for preview rendering
 * @param signal - An AbortSignal to cancel the compilation
 * @returns The compiled HTML and plain text, or throws on error
 */
export function compileEmail(
    source: string,
    previewProps: Record<string, unknown>,
    signal?: AbortSignal,
): Promise<CompileResult> {
    // Fail fast if already aborted
    if (signal?.aborted) {
        return Promise.reject(new DOMException("Aborted", "AbortError"))
    }

    const id = nextId++
    const w = getWorker()

    return new Promise<CompileResult>((resolve, reject) => {
        pending.set(id, { resolve, reject })

        // Wire up abort: remove the pending entry and reject
        const onAbort = () => {
            if (pending.delete(id)) {
                reject(new DOMException("Aborted", "AbortError"))
            }
        }
        signal?.addEventListener("abort", onAbort, { once: true })

        const msg: WorkerRequest = { id, source, previewProps }
        w.postMessage(msg)
    })
}
