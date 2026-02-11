import { useContext } from "react";
import { useGetPuck } from "@puckeditor/core";
import { TemplateWorkflowContext } from "../../../contexts";
import { CampaignContext, ProjectContext, TemplateContext } from "@/mod";
import api from "@/api";
import type CodeStore from "../CodeEditorPlugins/CodeStore";
import type CodeEditorEventListener from "../CodeEditorPlugins/CodeEditorEventListener";

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

    const updated = await api.campaigns.templates.update(
      project.id,
      campaign.id,
      template.id,
      {
        data: {
          ...template.data,
          editor: useRawHtml ? undefined : appState.data,
          rawHtml: useRawHtml ? props.codeStore.current : undefined,
          html: useRawHtml ? props.codeStore.current : template.data.html,
        },
      },
    );

    setTemplate(updated);
    return true;
  });

  return null;
}
