import { useContext } from "react";
import { Render, useGetPuck } from "@puckeditor/core";
import { TemplateWorkflowContext } from "../../../contexts";
import { CampaignContext, ProjectContext, TemplateContext } from "@/mod";
import api from "@/api";
import {
  Body,
  Font,
  Head,
  Html,
  pixelBasedPreset,
  render,
  Tailwind,
} from "@react-email/components";
import parse from "html-react-parser";
import { renderToString } from "react-dom/server";
import { config } from "../handlers/ConfigHandler";

export default function BlockSaveHandler() {
  const { onSubmit } = useContext(TemplateWorkflowContext);
  const [project] = useContext(ProjectContext);
  const [campaign] = useContext(CampaignContext);
  const [template, setTemplate] = useContext(TemplateContext);
  const getPuck = useGetPuck();

  onSubmit(async () => {
    const { appState } = getPuck();

    const content = renderToString(
      <Render config={config} data={appState.data} />,
    );

    const tailwindConfig = {
      presets: [pixelBasedPreset],
    };

    const html = await render(
      <Html lang={template.locale}>
        <Head>
          <Font
            fontFamily="Roboto"
            fallbackFontFamily="Verdana"
            webFont={{
              url: "https://fonts.gstatic.com/s/roboto/v27/KFOmCnqEu92Fr1Mu4mxKKTU1Kg.woff2",
              format: "woff2",
            }}
            fontWeight={400}
            fontStyle="normal"
          />
        </Head>
        <Tailwind config={tailwindConfig}>
          <Body>{parse(content)}</Body>
        </Tailwind>
      </Html>,
    );

    const updated = await api.campaigns.templates.update(
      project.id,
      campaign.id,
      template.id,
      {
        data: {
          ...template.data,
          editor: appState.data,
          html: html,
          type: "block",
        },
      },
    );

    setTemplate(updated);
    return true;
  });

  return null;
}
