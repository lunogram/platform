import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  Layout,
  Code2,
  ChevronLeft,
  Plus,
  LayoutTemplate,
} from "lucide-react";
import { Card } from "@/components/ui/card";
import { cn } from "@/utils";
import type { Template } from "../components/templates/PicingEmphasised/PricingEmphasised";
import { ChoiceCard } from "./ChoiceCard";

export interface TemplateProps {
  id: string;
  label: string;
  description: string;
  htmlComponent?: React.ReactNode;
  puckTemplate?: Template;
  thumbnail?: string;
}

interface EditorWizardProps {
  isOpen: boolean;
  onClose: () => void;
  onComplete: (type: "block" | "code", templateId: string) => void;
  templates: TemplateProps[];
}

export const EditorWizard = ({
  isOpen,
  onComplete,
  onClose,
  templates,
}: EditorWizardProps) => {
  const { t } = useTranslation();
  const [step, setStep] = useState<"type" | "template">("type");
  const [selectedType, setSelectedType] = useState<"block" | "code" | null>(null);

  const handleTypeSelect = (type: "block" | "code") => {
    setSelectedType(type);
    setStep("template");
  };

  const handleGoBack = () => {
    setStep("type");
    setSelectedType(null);
  };

  return (
    <Dialog onOpenChange={onClose} open={isOpen}>
      <DialogContent
        showClose={false}
        className="max-w-5xl h-[80vh] flex flex-col p-0 gap-0 overflow-hidden focus:ring-0 focus-visible:ring-0 focus:outline-none !border-none shadow-2xl"
      >
        {/* --- TOP BAR (Shadcn-themed) --- */}
        <div className="h-16 px-6 flex justify-between items-center border-b bg-muted/30 shrink-0">
          <div className="flex items-center gap-4">
            {step === "template" && (
              <Button
                variant="ghost"
                size="icon"
                onClick={handleGoBack}
                className="-ml-2 h-8 w-8 rounded-full"
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
            )}
            <div className="flex gap-1.5">
              <div className={cn("h-1.5 w-10 rounded-full transition-colors", step === "type" ? "bg-primary" : "bg-muted")} />
              <div className={cn("h-1.5 w-10 rounded-full transition-colors", step === "template" ? "bg-primary" : "bg-muted")} />
            </div>
          </div>
          
          <div className="flex items-center gap-4">
            <span className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground">
              {step === "type" ? t('campaign.template.editor.wizard.step1') : t('campaign.template.editor.wizard.step2')}
            </span>
          </div>
        </div>

        <div className="flex-1 flex flex-col min-h-0 bg-background">
          {step === "type" ? (
            <div className="flex-1 flex flex-col items-center justify-center p-10">
              <div className="w-full max-w-3xl space-y-10">
                <DialogHeader className="text-center space-y-3">
                  <DialogTitle className="text-4xl font-black tracking-tight">
                    {t('campaign.template.editor.wizard.title')}
                  </DialogTitle>
                  <DialogDescription className="text-base max-w-md">
                    {t('campaign.template.editor.wizard.description')}
                  </DialogDescription>
                </DialogHeader>
                
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                  <ChoiceCard
                    title={t('campaign.template.editor.wizard.visualBuilder')}
                    description={t('campaign.template.editor.wizard.visualBuilderDesc')}
                    icon={<Layout className="h-10 w-10" />}
                    onClick={() => handleTypeSelect("block")}
                    className="h-72 border-2"
                  />
                  <ChoiceCard
                    title={t('campaign.template.editor.wizard.developerMode')}
                    description={t('campaign.template.editor.wizard.developerModeDesc')}
                    icon={<Code2 className="h-10 w-10" />}
                    onClick={() => handleTypeSelect("code")}
                    className="h-72 border-2"
                  />
                </div>
              </div>
            </div>
          ) : (
            <div className="flex flex-col h-full">
              <DialogHeader className="p-8 pb-4 text-center space-y-2 shrink-0">
                <DialogTitle className="text-3xl font-black tracking-tight">
                  {t('campaign.template.editor.wizard.chooseTemplate')}
                </DialogTitle>
                <DialogDescription className="text-sm font-medium text-primary uppercase tracking-wider">
                  {t('campaign.template.editor.wizard.configuring')} {selectedType === "block" ? t('campaign.template.editor.wizard.visualBuilder') : t('campaign.template.editor.wizard.developerMode')}
                </DialogDescription>
              </DialogHeader>

              {/* Refinement: Replaced standard div with ScrollArea */}
              <div className="flex-1 px-8 pb-8 overflow-y-auto">
                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 max-w-5xl mx-auto py-2">
                  <ChoiceCard
                    variant="dashed"
                    title={t('campaign.template.editor.wizard.blankSlate')}
                    description={t('campaign.template.editor.wizard.noTemplate')}
                    icon={<Plus />}
                    onClick={() => onComplete(selectedType!, "blank")}
                    className="aspect-square sm:aspect-auto sm:h-full"
                  />
                  {templates.map((t) => (
                    <Card
                      key={t.id}
                      role="button"
                      onClick={() => onComplete(selectedType!, t.id)}
                      className="group overflow-hidden border-2 cursor-pointer hover:border-primary transition-colors"
                    >
                      <div className="aspect-video bg-muted flex items-center justify-center overflow-hidden">
                        {t.thumbnail ? (
                          <img src={t.thumbnail} className="h-full w-full object-cover group-hover:scale-105 transition-transform" />
                        ) : (
                          <LayoutTemplate className="h-10 w-10 text-muted-foreground/40" />
                        )}
                      </div>
                      <div className="p-3 border-t bg-background">
                        <h4 className="text-sm font-bold leading-none">{t.label}</h4>
                        <p className="text-[11px] text-muted-foreground line-clamp-1 mt-1.5">
                          {t.description}
                        </p>
                      </div>
                    </Card>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        {/* --- FOOTER (Proper shadcn component) --- */}
        <DialogFooter className="h-16 px-8 border-t bg-muted/50 flex flex-row items-center justify-between shrink-0 sm:justify-between">
          <div className="flex items-center gap-6 text-[11px] font-medium text-muted-foreground uppercase tracking-widest">
            <div className="flex items-center gap-2">
              <div className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
              {t('campaign.template.editor.wizard.autoSaveActive')}
            </div>
            <div className="flex items-center gap-2">
              <div className="h-1.5 w-1.5 rounded-full bg-blue-500" />
              {t('campaign.template.editor.wizard.cloudSync')}
            </div>
          </div>

          <div className="flex items-center gap-3">
            {step === "template" && (
              <span className="text-[10px] text-muted-foreground/60 font-bold italic">
                {t('campaign.template.editor.wizard.selectTemplate')}
              </span>
            )}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};