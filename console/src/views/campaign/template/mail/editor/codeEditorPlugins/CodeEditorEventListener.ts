import type { EditorChangeListener } from "../shared/types"
import DebouncedEvent from "./debouncedEvent"

class CodeEditorEventListener {
    private listeners = new Map<string, Set<EditorChangeListener>>()
    private debouncedEvent = new DebouncedEvent()

    public emit(event: string, data?: string) {
        this.listeners.get(event)?.forEach((listener) => listener(data))
    }

    public safeEmit(event: string, data?: string, delay = 400) {
        this.debouncedEvent.trigger<string[], (event: string, data?: string) => void>(
            () => this.emit(event, data),
            [event, data ? data : ""],
            delay,
        )
    }

    public addEventListener(event: string, listener: EditorChangeListener): () => void {
        if (!this.listeners.has(event)) {
            this.listeners.set(event, new Set([listener]))
        } else {
            this.listeners.get(event)!.add(listener)
        }

        return () => this.removeEventListener(event, listener)
    }

    public removeEventListener(event: string, listener?: EditorChangeListener) {
        if (listener) {
            this.listeners.get(event)?.delete(listener)
        } else {
            this.listeners.delete(event)
        }

        if (this.listeners.get(event)?.size === 0) {
            this.listeners.delete(event)
        }
    }
}

export default new CodeEditorEventListener()
