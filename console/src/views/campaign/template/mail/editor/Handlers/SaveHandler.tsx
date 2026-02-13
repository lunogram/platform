import { useContext } from "react";
import { Render, useGetPuck } from "@puckeditor/core";
import { TemplateWorkflowContext } from "../../../contexts";
import { CampaignContext, ProjectContext, TemplateContext } from "@/mod";
import api from "@/api";
import type CodeStore from "../CodeEditorPlugins/CodeStore";
import type CodeEditorEventListener from "../CodeEditorPlugins/CodeEditorEventListener";
import {
  Body,
  pixelBasedPreset,
  render,
  Tailwind,
} from "@react-email/components";
import parse from "html-react-parser";
import { renderToString } from "react-dom/server";
import { config } from "./ConfigHandler";

export default function SaveHandler(props: {
  eventListener: typeof CodeEditorEventListener;
  codeStore: typeof CodeStore;
}) {
  const { onSubmit } = useContext(TemplateWorkflowContext);
  const [project] = useContext(ProjectContext);
  const [campaign] = useContext(CampaignContext);
  const [template, setTemplate] = useContext(TemplateContext);
  const getPuck = useGetPuck();

  onSubmit(async () => {
    const { appState } = getPuck();
    const useRawHtml = props.codeStore.current.trim().length > 0;

    const content = renderToString(
      <Render config={config} data={appState.data} />,
    );

    const tailwindConfig = {
      presets: [pixelBasedPreset],
    };

    const html = await render(
      <Tailwind config={tailwindConfig}>
        <Body>{parse(content)}</Body>
      </Tailwind>,
    );

    const updated = await api.campaigns.templates.update(
      project.id,
      campaign.id,
      template.id,
      {
        data: {
          ...template.data,
          editor: useRawHtml ? undefined : appState.data,
          rawHtml: useRawHtml ? props.codeStore.current : undefined,
          html: useRawHtml ? props.codeStore.current : html,
        },
      },
    );

    props.codeStore.setCode("");

    setTemplate(updated);
    return true;
  });

  return null;
}
