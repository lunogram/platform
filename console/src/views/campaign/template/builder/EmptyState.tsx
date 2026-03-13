import { useCallback } from "react"
import { Sparkles } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useBuilderActions, useBuilderStream } from "./useBuilder"
import { PromptInput } from "./PromptInput"

const SUGGESTIONS = [
    { label: "Welcome email", prompt: "Create a welcome email for new users" },
    { label: "Promotional offer", prompt: "Create a promotional email with a 25% discount offer" },
    { label: "Weekly newsletter", prompt: "Create a weekly newsletter digest with articles" },
    { label: "Re-engagement", prompt: "Create a re-engagement email for inactive users" },
]

interface EmptyStateProps {
    onAddImages?: () => void
}

export function EmptyState({ onAddImages }: EmptyStateProps) {
    const { sendMessage } = useBuilderActions()
    const { isAgentTyping } = useBuilderStream()

    const handleSuggestion = useCallback(
        (prompt: string) => {
            sendMessage(prompt)
        },
        [sendMessage],
    )

    return (
        <div className="flex flex-col items-center justify-center h-full px-6">
            <div className="w-full max-w-xl flex flex-col items-center gap-8">
                {/* Icon + heading */}
                <div className="text-center">
                    <div className="w-14 h-14 mx-auto mb-4 rounded-2xl bg-primary/10 flex items-center justify-center">
                        <Sparkles className="w-7 h-7 text-primary" />
                    </div>
                    <h2 className="text-xl font-semibold text-foreground mb-2 inline-flex items-center gap-2 justify-center">
                        Build your email template
                        <Badge className="text-[10px] font-medium uppercase tracking-wider px-1.5 py-0 bg-violet-500/15 text-violet-600 dark:text-violet-400 border-violet-500/25 hover:bg-violet-500/15">
                            Alpha
                        </Badge>
                    </h2>
                    <p className="text-sm text-muted-foreground max-w-md">
                        Describe the email you want to create, and the AI will generate a
                        production-ready template you can iterate on.
                    </p>
                </div>

                {/* Prompt input */}
                <div className="w-full">
                    <PromptInput
                        autoFocus
                        onAddImages={onAddImages}
                        placeholder="Describe the email you want to create..."
                        imageButtonVariant="ghost"
                        pillsClassName="mb-2 justify-center"
                        textareaClassName="flex-1 resize-none bg-transparent px-2 py-1.5 text-sm placeholder:text-muted-foreground focus-visible:outline-none disabled:opacity-50"
                        inputRowClassName="flex items-end gap-2 border rounded-xl bg-background shadow-sm p-2"
                    />
                </div>

                {/* Suggestion chips */}
                <div className="flex flex-wrap gap-2 justify-center">
                    {SUGGESTIONS.map((s) => (
                        <Button
                            key={s.label}
                            type="button"
                            variant="outline"
                            size="sm"
                            className="rounded-full"
                            onClick={() => handleSuggestion(s.prompt)}
                            disabled={isAgentTyping}
                        >
                            {s.label}
                        </Button>
                    ))}
                </div>
            </div>
        </div>
    )
}
