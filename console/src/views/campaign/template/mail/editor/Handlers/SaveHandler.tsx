import { useContext } from "react";
import { Render, useGetPuck } from "@puckeditor/core";
import {
  Body,
  Font,
  Head,
  Html,
  Tailwind,
  pixelBasedPreset,
} from "@react-email/components";
import { render } from "@react-email/render";
import { renderToString } from "react-dom/server";
import parse from "html-react-parser";
import { TemplateWorkflowContext } from "../../../contexts";
import { CampaignContext, ProjectContext, TemplateContext } from "@/mod";
import { config } from "./ConfigHandler";
import api from "@/api";
import { rawHtmlState } from "../editorEvents";

const TAILWIND_CONFIG = {
  presets: [pixelBasedPreset],
};

const ROBOTO_FONT = {
  url: "https://fonts.gstatic.com/s/roboto/v27/KFOmCnqEu92Fr1Mu4mxKKTU1Kg.woff2",
  format: "woff2" as const,
};

export default function SaveHandler() {
  const { onSubmit } = useContext(TemplateWorkflowContext);
  const [project] = useContext(ProjectContext);
  const [campaign] = useContext(CampaignContext);
  const [template, setTemplate] = useContext(TemplateContext);
  const getPuck = useGetPuck();

  onSubmit(async () => {
    const { appState } = getPuck();

    // If raw HTML has content, use that instead of drag-and-drop
    const useRawHtml = rawHtmlState.html.trim().length > 0;

    const content = useRawHtml
      ? rawHtmlState.html
      : renderToString(<Render config={config} data={appState.data} />);

    const html = await render(
      <Html lang={template.locale}>
        <Head>
          <Font
            fontFamily="Roboto"
            fallbackFontFamily="Verdana"
            webFont={ROBOTO_FONT}
            fontWeight={400}
            fontStyle="normal"
          />
        </Head>
        <Tailwind config={TAILWIND_CONFIG}>
          <Body>{parse(content)}</Body>
        </Tailwind>
      </Html>,
      { pretty: true },
    );

    const updated = await api.campaigns.templates.update(
      project.id,
      campaign.id,
      template.id,
      {
        data: {
          ...template.data,
          // Clear editor data if using raw HTML, otherwise keep it
          editor: useRawHtml ? undefined : appState.data,
          rawHtml: useRawHtml ? rawHtmlState.html : undefined,
          html,
        },
      },
    );

    setTemplate(updated);
    return true;
  });

  return null;
}
