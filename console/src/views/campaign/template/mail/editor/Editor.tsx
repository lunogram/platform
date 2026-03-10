import { useContext, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"

import "@puckeditor/core/dist/index.css"
import { TemplateContext } from "@/contexts"
import { TemplateWorkflowContext } from "../../contexts"

import { EditorWizard } from "./SelectionModals/EditorWizard"
import { HtmlEditor } from "./htmlEditor/HtmlEditor"
import { BlockEditor } from "./blockEditor/BlockEditor"
import CodeEditorEventListener from "./CodeEditorPlugins/CodeEditorEventListener"
import CodeStore from "./CodeEditorPlugins/CodeStore"
import type { EmailTemplate } from "@/types"

export default function Editor() {
    const [template] = useContext(TemplateContext)
    const { setCanProceed } = useContext(TemplateWorkflowContext)
    const initialMode = template?.data?.rawHtml ? "code" : template?.data?.editor ? "block" : null

    const [editorMode, setEditorMode] = useState<"block" | "code" | null>(initialMode)

    const showWizard = editorMode === null

    useEffect(() => {
        setCanProceed(!showWizard)
        return () => {
            setCanProceed(true)
        }
    }, [showWizard, setCanProceed])

    useEffect(() => {
        return () => {
            CodeStore.setCode("")
        }
    }, [])

    const handleComplete = (type: "block" | "code", selectedTemplate?: EmailTemplate) => {
        setEditorMode(type)

        if (selectedTemplate) {
            if (type === "code" && selectedTemplate.html) {
                template.data.html = selectedTemplate.html
                CodeStore.setCode(selectedTemplate.html)
                CodeEditorEventListener.emit("CODE_CHANGE")
            } else if (type === "block" && selectedTemplate.blocks) {
                template.data.editor = selectedTemplate.blocks
            }
        } else if (type === "code") {
            // Provide a basic HTML skeleton for blank slate in Developer Mode
            const blankHtmlSkeleton = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
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
      <p>Hello {{user.data.full_name}},</p>
      <p>Your email content goes here.</p>
    </div>
    <div class="email-footer">
      <p>
        <a href="{{unsubscribe_url}}">Unsubscribe</a> |
        <a href="{{preferences_url}}">Email Preferences</a>
      </p>
    </div>
  </div>
</body>
</html>`
            template.data.html = blankHtmlSkeleton
            CodeStore.setCode(blankHtmlSkeleton)
            CodeEditorEventListener.emit("CODE_CHANGE")
        }
    }

    const data = template?.data?.editor ?? { content: [], root: {} }

    return (
        <div className="w-full h-full flex flex-col">
            {showWizard ? (
                <EditorWizard onComplete={handleComplete} />
            ) : (
                <>
                    {editorMode === "code" ? (
                        <HtmlEditor
                            data={data}
                            html={template.data.rawHtml || template.data.html}
                        />
                    ) : (
                        <BlockEditor data={data} />
                    )}
                </>
            )}
        </div>
    )
}
