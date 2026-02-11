import { useState } from "react";
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
  Loader2,
} from "lucide-react";
import { ChoiceCard } from "./ChoiceCard";
import { Card } from "@/components/ui/card";
import { cn } from "@/utils";

export interface Template {
  id: string;
  label: string;
  description: string;
  thumbnail?: string;
}

interface EditorWizardProps {
  isOpen: boolean;
  onClose: () => void;
  onComplete: (type: "block" | "code", templateId: string) => void;
  templates: Template[];
  isLoading?: boolean;
}

export const EditorWizard = ({
  isOpen,
  onComplete,
  templates,
  isLoading,
}: EditorWizardProps) => {
  const [step, setStep] = useState<"type" | "template">("type");
  const [selectedType, setSelectedType] = useState<"block" | "code" | null>(
    null,
  );

  const handleTypeSelect = (type: "block" | "code") => {
    setSelectedType(type);
    setStep("template");
  };

  const handleGoBack = () => {
    setStep("type");
    setSelectedType(null);
  };

  return (
    <Dialog open={isOpen}>
      <DialogContent
        className={cn(
          "transition-all duration-300 ease-in-out",
          step === "type" ? "max-w-lg" : "max-w-4xl",
        )}
      >
        {/* Step 1: Editor Type Selection */}
        {step === "type" && (
          <div className="animate-in fade-in zoom-in-95 duration-200">
            <DialogHeader>
              <DialogTitle className="text-2xl font-bold">
                New Project
              </DialogTitle>
              <DialogDescription>
                Choose how you want to build your page.
              </DialogDescription>
            </DialogHeader>
            <div className="grid grid-cols-1 gap-4 py-6 sm:grid-cols-2">
              <ChoiceCard
                title="Visual Builder"
                description="Drag and drop blocks visually"
                icon={<Layout />}
                onClick={() => handleTypeSelect("block")}
              />
              <ChoiceCard
                title="Developer Mode"
                description="Write raw HTML for full control"
                icon={<Code2 />}
                onClick={() => handleTypeSelect("code")}
              />
            </div>
          </div>
        )}

        {/* Step 2: Template Selection */}
        {step === "template" && (
          <div className="animate-in fade-in slide-in-from-right-4 duration-300">
            <DialogHeader className="flex-row items-center gap-4 space-y-0 border-b pb-4">
              <Button
                variant="ghost"
                size="icon"
                onClick={handleGoBack}
                className="h-8 w-8 rounded-full"
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <div>
                <DialogTitle>Select a Template</DialogTitle>
                <DialogDescription>
                  Start with a layout or a blank canvas
                </DialogDescription>
              </div>
            </DialogHeader>

            <div className="mt-4 max-h-[60vh] pr-4 overflow-y-auto">
              {isLoading ? (
                <div className="flex h-40 items-center justify-center">
                  <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                </div>
              ) : (
                <div className="grid grid-cols-1 gap-4 p-1 sm:grid-cols-2 md:grid-cols-3">
                  <ChoiceCard
                    variant="dashed"
                    title="Blank Slate"
                    description="No template"
                    icon={<Plus />}
                    onClick={() => onComplete(selectedType!, "blank")}
                    className="h-full"
                  />
                  {templates.map((t) => (
                    <Card
                      key={t.id}
                      role="button"
                      onClick={() => onComplete(selectedType!, t.id)}
                      className="group overflow-hidden transition-all hover:border-primary active:scale-[0.98]"
                    >
                      <div className="aspect-video bg-muted flex items-center justify-center overflow-hidden">
                        {t.thumbnail ? (
                          <img
                            src={t.thumbnail}
                            alt={t.label}
                            className="h-full w-full object-cover"
                          />
                        ) : (
                          <LayoutTemplate className="h-10 w-10 text-muted-foreground/50" />
                        )}
                      </div>
                      <div className="p-3 border-t">
                        <h4 className="text-sm font-bold">{t.label}</h4>
                        <p className="text-[11px] text-muted-foreground line-clamp-1">
                          {t.description}
                        </p>
                      </div>
                    </Card>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
        <DialogFooter className="border-t pt-4">
          Note: You can not switch editing modes after making a selection.
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
