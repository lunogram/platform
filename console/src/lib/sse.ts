export interface SSECallbacks<T extends Record<string, unknown>> {
    onEvent: (event: string, data: T) => void
    onError: (error: Error) => void
    onClose: () => void
}

export async function fetchSSE<T extends Record<string, unknown>>(
    url: string,
    options: RequestInit,
    callbacks: SSECallbacks<T>,
): Promise<void> {
    const response = await fetch(url, {
        ...options,
        headers: {
            ...options.headers,
            Accept: "text/event-stream",
            "Content-Type": "application/json",
        },
        credentials: "include",
    })

    if (!response.ok) {
        throw new Error(`SSE request failed: ${response.status}`)
    }

    const reader = response.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ""

    try {
        while (true) {
            const { done, value } = await reader.read()
            if (done) break

            buffer += decoder.decode(value, { stream: true })
            const lines = buffer.split("\n")
            buffer = lines.pop() ?? ""

            let currentEvent = ""
            let dataLines: string[] = []
            for (const line of lines) {
                if (line.startsWith("event: ")) {
                    currentEvent = line.slice(7)
                } else if (line.startsWith("data: ")) {
                    dataLines.push(line.slice(6))
                } else if (line === "") {
                    if (dataLines.length > 0) {
                        try {
                            const data = JSON.parse(dataLines.join("\n")) as T
                            callbacks.onEvent(currentEvent, data)
                        } catch {
                            // skip malformed events
                        }
                    }
                    currentEvent = ""
                    dataLines = []
                }
            }
        }
    } catch (err) {
        callbacks.onError(err instanceof Error ? err : new Error(String(err)))
        return
    }

    callbacks.onClose()
}
