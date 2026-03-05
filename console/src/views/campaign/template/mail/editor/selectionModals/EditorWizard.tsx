import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Layout, Code2, ArrowLeft, Plus, LayoutTemplate } from "lucide-react"
import { Card } from "@/components/ui/card"
import { cn } from "@/utils"
import type { Template } from "../components/templates/PicingEmphasised/PricingEmphasised"
import { ChoiceCard } from "./ChoiceCard"

export interface TemplateProps {
    id: string
    label: string
    description: string
    htmlComponent?: React.ReactNode
    puckTemplate?: Template
    thumbnail?: string
}

interface EditorWizardProps {
    onComplete: (type: "block" | "code", templateId: string) => void
    templates: TemplateProps[]
}

export const EditorWizard = ({ onComplete, templates }: EditorWizardProps) => {
    const { t } = useTranslation()
    const [step, setStep] = useState<"type" | "template">("type")
    const [selectedType, setSelectedType] = useState<"block" | "code" | null>(null)

    const handleTypeSelect = (type: "block" | "code") => {
        setSelectedType(type)
        setStep("template")
    }

    const handleGoBack = () => {
        setStep("type")
        setSelectedType(null)
    }

    return (
        <div className="flex-1 flex flex-col p-8 overflow-hidden">
            {step === "type" ? (
                <>
                    <div className="mb-6 shrink-0">
                        <h1 className="text-2xl font-semibold">
                            {t("campaign.template.editor.wizard.title")}
                        </h1>
                        <p className="text-muted-foreground">
                            {t("campaign.template.editor.wizard.description")}
                        </p>
                    </div>

                    <div className="flex-1 flex items-center justify-center">
                        <div className="grid grid-cols-2 gap-6 w-full max-w-xl">
                            <ChoiceCard
                                title={t("campaign.template.editor.wizard.visualBuilder")}
                                description={t("campaign.template.editor.wizard.visualBuilderDesc")}
                                icon={<Layout />}
                                onClick={() => handleTypeSelect("block")}
                            />
                            <ChoiceCard
                                title={t("campaign.template.editor.wizard.developerMode")}
                                description={t("campaign.template.editor.wizard.developerModeDesc")}
                                icon={<Code2 />}
                                onClick={() => handleTypeSelect("code")}
                            />
                        </div>
                    </div>
                </>
            ) : (
                <>
                    <div className="flex items-center gap-3 mb-6 shrink-0">
                        <Button
                            variant="ghost"
                            size="icon"
                            onClick={handleGoBack}
                            className="h-8 w-8 shrink-0"
                        >
                            <ArrowLeft className="h-4 w-4" />
                        </Button>
                        <div>
                            <h1 className="text-2xl font-semibold">
                                {t("campaign.template.editor.wizard.chooseTemplate")}
                            </h1>
                            <p className="text-muted-foreground">
                                {selectedType === "block"
                                    ? t("campaign.template.editor.wizard.visualBuilder")
                                    : t("campaign.template.editor.wizard.developerMode")}
                            </p>
                        </div>
                    </div>

                    <div className="flex-1 overflow-y-auto">
                        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                            <Card
                                role="button"
                                onClick={() => onComplete(selectedType!, "blank")}
                                className={cn(
                                    "flex flex-col items-center justify-center cursor-pointer",
                                    "border-dashed hover:border-primary hover:bg-accent transition-colors",
                                    "aspect-4/3 min-h-48",
                                )}
                            >
                                <div className="rounded-md bg-muted p-3 mb-2">
                                    <Plus className="h-5 w-5 text-muted-foreground" />
                                </div>
                                <span className="text-sm font-medium">
                                    {t("campaign.template.editor.wizard.blankSlate")}
                                </span>
                                <span className="text-xs text-muted-foreground mt-1 text-center">
                                    {t("campaign.template.editor.wizard.noTemplate")}
                                </span>
                            </Card>

                            {templates.map((template) => (
                                <Card
                                    key={template.id}
                                    role="button"
                                    onClick={() => onComplete(selectedType!, template.id)}
                                    className="group cursor-pointer hover:border-primary transition-colors aspect-4/3 min-h-48 flex flex-col"
                                >
                                    <div className="flex-1 bg-muted flex items-center justify-center overflow-hidden">
                                        {template.thumbnail ? (
                                            <img
                                                src={template.thumbnail}
                                                alt={template.label}
                                                className="h-full w-full object-cover group-hover:scale-105 transition-transform"
                                            />
                                        ) : (
                                            <LayoutTemplate className="h-8 w-8 text-muted-foreground/40" />
                                        )}
                                    </div>
                                    <div className="p-3 border-t">
                                        <p className="text-sm font-medium truncate">
                                            {template.label}
                                        </p>
                                        <p className="text-xs text-muted-foreground truncate mt-1">
                                            {template.description}
                                        </p>
                                    </div>
                                </Card>
                            ))}
                        </div>
                    </div>
                </>
            )}
        </div>
    )
}
