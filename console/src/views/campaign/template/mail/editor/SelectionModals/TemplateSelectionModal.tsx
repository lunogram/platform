import React from "react"
import { LayoutTemplate, Loader2 } from "lucide-react"
import { ChoiceCard } from "./ChoiceCard"

export interface Template {
    id: string
    label: string
    description: string
    thumbnail?: string
}

interface TemplateSelectionModalProps {
    isOpen: boolean
    templates: Template[]
    isLoading?: boolean
    onSelect: (templateId: string | "blank") => void
}

export const TemplateSelectionModal: React.FC<TemplateSelectionModalProps> = ({
    isOpen,
    templates,
    isLoading = false,
    onSelect,
}) => {
    if (!isOpen) return null

    return (
        <div className="fixed inset-0 z-9999 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
            <div className="relative flex h-full max-h-[85vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl">
                <div className="flex items-center justify-between border-b border-gray-100 px-8 py-5">
                    <div>
                        <h2 className="text-2xl font-bold text-gray-900">Select Template</h2>
                        <p className="text-sm text-gray-500">
                            Choose a starting point for your project.
                        </p>
                    </div>
                </div>

                <div className="flex-1 overflow-y-auto p-8">
                    {isLoading ? (
                        <div className="flex h-64 flex-col items-center justify-center gap-4">
                            <Loader2 className="h-8 w-8 animate-spin text-blue-600" />
                            <p className="text-gray-500">Loading library...</p>
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
                            <ChoiceCard
                                variant="dashed"
                                title="Blank Slate"
                                description="Start from scratch"
                                icon={<LayoutTemplate size={40} />}
                                onClick={() => onSelect("blank")}
                            />

                            {templates.map((t) => (
                                <ChoiceCard
                                    key={t.id}
                                    title={t.label}
                                    description={t.description}
                                    icon={<LayoutTemplate size={40} />}
                                    onClick={() => onSelect(t.id)}
                                />
                            ))}
                        </div>
                    )}
                </div>

                <div className="bg-gray-50 px-8 py-4 border-t border-gray-100 text-xs text-gray-400">
                    Templates include pre-configured blocks and layout settings.
                </div>
            </div>
        </div>
    )
}
