import { useContext, useState } from "react";

import "@puckeditor/core/dist/index.css";
import "./Editor.css";
import { TemplateContext } from "@/contexts";

import CodeStore from "./CodeEditorPlugins/CodeStore";
import { EditorWizard, type Template } from "./SelectionModals/EditorWizard";
import { HtmlEditor } from "./HtmlEditor";
import { BlockEditor } from "./BlockEditor";

const TESTING_TEMPLATES: Template[] = [
  {
    id: "welcome-email",
    label: "Welcome Email",
    description: "A professional welcome email template for new users",
    thumbnail: undefined,
  },
  {
    id: "promotional",
    label: "Promotional Offer",
    description: "Eye-catching template for promotional campaigns",
    thumbnail: undefined,
  },
  {
    id: "newsletter",
    label: "Newsletter",
    description: "Clean and simple newsletter layout",
    thumbnail: undefined,
  },
  {
    id: "confirmation",
    label: "Order Confirmation",
    description: "Template for order confirmation emails",
    thumbnail: undefined,
  },
  {
    id: "password-reset",
    label: "Password Reset",
    description: "Secure password reset request template",
    thumbnail: undefined,
  },
  {
    id: "discount-code",
    label: "Discount Code",
    description: "Exclusive discount offer for loyal customers",
    thumbnail: undefined,
  },
  {
    id: "event-invitation",
    label: "Event Invitation",
    description: "Professional event invitation template",
    thumbnail: undefined,
  },
  {
    id: "feedback-survey",
    label: "Feedback Survey",
    description: "Customer feedback and survey request",
    thumbnail: undefined,
  },
  {
    id: "re-engagement",
    label: "Re-engagement Campaign",
    description: "Win back inactive customers",
    thumbnail: undefined,
  },
  {
    id: "vip-exclusive",
    label: "VIP Exclusive Offer",
    description: "Premium offer for VIP members",
    thumbnail: undefined,
  },
];

export default function Editor() {
  const [template] = useContext(TemplateContext);
  const [isWizardOpen, setIsWizardOpen] = useState(true);
  const [editorMode, setEditorMode] = useState<"block" | "code" | null>(null);

  const handleComplete = (type: "block" | "code") => {
    setEditorMode(type);
    setIsWizardOpen(false);
  };

  const data = template.data.editor ?? { content: [], root: {} };

  if (template.data.html) {
    CodeStore.setCode(template.data.html);
    // handleComplete("code");
  }

  return (
    <div className="w-full h-full">
      <EditorWizard
        isOpen={isWizardOpen}
        onClose={() => setIsWizardOpen(false)}
        templates={TESTING_TEMPLATES}
        onComplete={handleComplete}
      />

      {!isWizardOpen && (
        <>
          {editorMode === "code" ? (
            <HtmlEditor data={data} html={template.data.html} />
          ) : (
            <BlockEditor data={data} />
          )}
        </>
      )}
    </div>
  );
}
