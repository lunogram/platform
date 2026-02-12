import { useContext, useState } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import "@puckeditor/core/dist/index.css";
import "./Editor.css";
import { TemplateContext } from "@/contexts";

import { EditorWizard, type Template } from "./SelectionModals/EditorWizard";
import { HtmlEditor } from "./HtmlEditor";
import { BlockEditor } from "./BlockEditor";
import CodeEditorEventListener from "./CodeEditorPlugins/CodeEditorEventListener";
import CodeStore from "./CodeEditorPlugins/CodeStore";
import { PricingEmphasisedHtml } from "./components/templates/PicingEmphasised/PricingEmphasisedHtml";
import { PricingEmphasised } from "./components/templates/PicingEmphasised/PricingEmphasised";

const TESTING_TEMPLATES: Template[] = [
  {
    id: "pricing-emphasized",
    label: "Pricing Emphasized",
    description: "A professional dual-plan pricing section for emails.",
    // You would put a screenshot of the component here
    thumbnail: undefined,
  },
];

export default function Editor() {
  const [template] = useContext(TemplateContext);
  console.log("Current template data:", template?.data);
  const initialMode = template?.data?.rawHtml
    ? "code"
    : template?.data?.editor
      ? "block"
      : null;

  const [editorMode, setEditorMode] = useState<"block" | "code" | null>(
    initialMode,
  );
  const [isWizardOpen, setIsWizardOpen] = useState(initialMode === null);

  const handleComplete = (type: "block" | "code", templateId: string) => {
    setEditorMode(type);
    setIsWizardOpen(false);

    // if (templateId === "pricing-emphasized") {
    //   if (type === "code") {
    //     template.data.html = renderToStaticMarkup(<PricingEmphasisedHtml />);
    //     CodeStore.setCode(template.data.html);
    //     CodeEditorEventListener.emit("CODE_CHANGE");
    //   } else {
    //     template.data.editor = {
    //       content: [
    //         {
    //           id: "pricing-emphasised",
    //           type: "pricing-emphasised",
    //           content: { ...PricingEmphasised.defaultProps },
    //         },
    //       ],
    //       root: {},
    //     };

    //     console.log("Set block editor data:", template.data.editor);
    //   }
    // }
  };

  const data = template?.data?.editor ?? { content: [], root: {} };

  return (
    <div className="w-full h-full">
      <EditorWizard
        isOpen={isWizardOpen}
        onClose={() => setIsWizardOpen(false)}
        templates={TESTING_TEMPLATES}
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
