import { useContext, useEffect, useState } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { useTranslation } from "react-i18next";

import "@puckeditor/core/dist/index.css";
import { TemplateContext } from "@/contexts";
import { TemplateWorkflowContext } from "../../contexts";

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

function EmailTemplates(): TemplateProps[] {
  const { t } = useTranslation();

  return [
    {
      id: "pricing-emphasized",
      label: t("campaign.template.email.templates.pricingEmphasized.label"),
      description: t(
        "campaign.template.email.templates.pricingEmphasized.description",
      ),
      htmlComponent: <PricingEmphasisedHtml />,
      puckTemplate: PricingEmphasisedTemplate,
      // You would put a screenshot of the component here
      thumbnail: undefined,
    },
  ];
}

export default function Editor() {
  const [template] = useContext(TemplateContext);
  const { setCanProceed } = useContext(TemplateWorkflowContext);
  const emailTemplates = EmailTemplates();
  const initialMode = template?.data?.rawHtml
    ? "code"
    : template?.data?.editor
      ? "block"
      : null;

  const [editorMode, setEditorMode] = useState<"block" | "code" | null>(
    initialMode,
  );

  const showWizard = editorMode === null;

  useEffect(() => {
    setCanProceed(!showWizard);
    return () => {
      setCanProceed(true);
    };
  }, [showWizard, setCanProceed]);

  useEffect(() => {
    return () => {
      CodeStore.setCode("");
    };
  }, []);

  const handleComplete = (type: "block" | "code", templateId: string) => {
    setEditorMode(type);

    const selectedTemplate = emailTemplates.find((t) => t.id === templateId);
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
    } else if (type === "code" && templateId === "blank") {
      // Provide a basic HTML skeleton for blank slate in Developer Mode
      const blankHtmlSkeleton = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{campaign.subject}}</title>
  <style>
    body {
      margin: 0;
      padding: 0;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
      line-height: 1.6;
      color: #333333;
      background-color: #f4f4f4;
    }
    .email-container {
      max-width: 600px;
      margin: 0 auto;
      background-color: #ffffff;
    }
    .email-header {
      padding: 20px;
      text-align: center;
    }
    .email-body {
      padding: 20px;
    }
    .email-footer {
      padding: 20px;
      text-align: center;
      font-size: 12px;
      color: #666666;
    }
  </style>
</head>
<body>
  <div class="email-container">
    <div class="email-header">
      <!-- Header content -->
    </div>
    <div class="email-body">
      <p>Hello {{user.first_name}},</p>
      <p>Your email content goes here.</p>
    </div>
    <div class="email-footer">
      <p>
        <a href="{{unsubscribe_url}}">Unsubscribe</a> |
        <a href="{{preferences_url}}">Email Preferences</a>
      </p>
      <p><a href="{{web_version_url}}">View in browser</a></p>
    </div>
  </div>
</body>
</html>`;
      template.data.html = blankHtmlSkeleton;
      CodeStore.setCode(blankHtmlSkeleton);
      CodeEditorEventListener.emit("CODE_CHANGE");
    }
  };

  const data = template?.data?.editor ?? { content: [], root: {} };

  return (
    <div className="w-full h-full flex flex-col">
      {showWizard ? (
        <EditorWizard templates={emailTemplates} onComplete={handleComplete} />
      ) : (
        <>
          {editorMode === "code" ? (
            <HtmlEditor data={data} html={template.data.rawHtml || template.data.html} />
          ) : (
            <BlockEditor data={data} />
          )}
        </>
      )}
    </div>
  );
}
