import { Puck, Render, useGetPuck, type Config } from "@measured/puck";
import { pixelBasedPreset, Tailwind, Html, Head, Body } from "@react-email/components";
import { render, pretty } from "@react-email/render";
import { viewports } from "./viewport";
import { useContext } from "react";
import { CampaignWorkflowContext } from "../../../contexts";

import { Button, type ButtonProps } from "./components/Button";

import "@measured/puck/puck.css";
import "./Editor.css";
import { ProjectContext, TemplateContext } from "@/contexts";
import api from "@/api";

interface Components {
    Button: ButtonProps
}

const config: Config<Components> = {
    categories: {},
    root: {
        fields: {},
        render: ({ children }) => {
            const config = {
                presets: [pixelBasedPreset],
            }

            return (
                <Tailwind config={config}>
                    {children}
                </Tailwind>

            );
        },
    },
    components: {
        Button,
    },
}


// The SaveHandler component registers a save handler that is called when the
// user clicks the "Next" button in the CampaignDetail view. This handler has to
// be defined within a separate component to have access to the Puck context.
function SaveHandler() {
    const { onSubmit } = useContext(CampaignWorkflowContext);
    const [project] = useContext(ProjectContext);
    const [template, setTemplate] = useContext(TemplateContext);
    const getPuck = useGetPuck();

    onSubmit(async () => {
        const { appState } = getPuck();
        const html = await render((
            <Html lang={template.locale}>
                <Head />
                <Body>
                    <Render config={config} data={appState.data} />
                </Body>
            </Html>
        ));

        const updated = await api.templates.update(project.id, template.id, {
            data: {
                ...template.data,
                editor: appState.data,
                html,
            }
        })

        setTemplate(updated);
        return true
    });

    return null;
}

export default function Editor() {
    const [template] = useContext(TemplateContext);
    const data = template.data.editor ?? {}

    return (
        <div className="w-full h-full">
            <Puck viewports={viewports} config={config} data={data} overrides={{
                puck: ({ children }) => (
                    <>
                        <SaveHandler />
                        {children}
                    </>
                ),
            }} />
        </div>
    );
}
