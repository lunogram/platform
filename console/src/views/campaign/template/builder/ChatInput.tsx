import { useBuilderThread } from "./useBuilder"
import { PromptInput } from "./PromptInput"

interface ChatInputProps {
    /** Autofocus the input on mount */
    autoFocus?: boolean
    /** Callback to open the image library modal */
    onAddImages?: () => void
}

export function ChatInput({ autoFocus = true, onAddImages }: ChatInputProps) {
    const { messages } = useBuilderThread()

    return (
        <div className="border-t bg-background p-4">
            <PromptInput
                autoFocus={autoFocus}
                onAddImages={onAddImages}
                placeholder={
                    messages.length === 0
                        ? "Describe the email you want to create..."
                        : "Tell me what to change..."
                }
                pillsClassName="mb-4"
            />
        </div>
    )
}
