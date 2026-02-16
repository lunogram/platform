import { useContext, useEffect, useState } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import "@puckeditor/core/dist/index.css";
import { TemplateContext } from "@/contexts";

import {
  EditorWizard,
  type TemplateProps,
} from "./selectionModals/EditorWizard";
import { HtmlEditor } from "./htmlEditor/HtmlEditor";
import { BlockEditor } from "./blockEditor/BlockEditor";
import CodeEditorEventListener from "./codeEditorPlugins/CodeEditorEventListener";
import CodeStore from "./codeEditorPlugins/CodeStore";
import { PricingEmphasisedHtml } from "./components/templates/PicingEmphasised/PricingEmphasisedHtml";
import { PricingEmphasisedTemplate } from "./components/templates/PicingEmphasised/PricingEmphasised";

const EMAIL_TEMPLATES: TemplateProps[] = [
  {
    id: "pricing-emphasized",
    label: "Pricing Emphasized",
    description: "A professional dual-plan pricing section for emails.",
    htmlComponent: <PricingEmphasisedHtml />,
    puckTemplate: PricingEmphasisedTemplate,
    // You would put a screenshot of the component here
    thumbnail: undefined,
  },
];

export default function Editor() {
  const [template] = useContext(TemplateContext);
  const initialMode = template?.data?.rawHtml
    ? "code"
    : template?.data?.editor
      ? "block"
      : null;

  const [editorMode, setEditorMode] = useState<"block" | "code" | null>(
    initialMode,
  );

  const [isWizardOpen, setIsWizardOpen] = useState(initialMode === null);

  useEffect(() => {
    return () => {
      CodeStore.setCode("");
    };
  }, []);

  const handleComplete = (type: "block" | "code", templateId: string) => {
    setEditorMode(type);
    setIsWizardOpen(false);

    const selectedTemplate = EMAIL_TEMPLATES.find((t) => t.id === templateId);
    if (selectedTemplate) {
      if (type === "code") {
        template.data.html = renderToStaticMarkup(
          selectedTemplate.htmlComponent,
        );
        CodeStore.setCode(template.data.html);
        CodeEditorEventListener.emit("CODE_CHANGE");
      } else {
        template.data.editor = selectedTemplate.puckTemplate;
      }
    }
  };

  const data = template?.data?.editor ?? { content: [], root: {} };

  return (
    <div className="w-full h-full">
      <EditorWizard
        isOpen={isWizardOpen}
        onClose={() => setIsWizardOpen(false)}
        templates={EMAIL_TEMPLATES}
        onComplete={handleComplete}
      />

      {!isWizardOpen && editorMode && (
        <>
          {editorMode === "code" ? (
            <HtmlEditor data={data} html={template.data.rawHtml} />
          ) : (
            <BlockEditor data={data} />
          )}
        </>
      )}
    </div>
  );
}
