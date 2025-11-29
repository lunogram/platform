/* eslint-disable react-hooks/rules-of-hooks */
import { Puck, Render, useGetPuck, type Config } from "@measured/puck";
import { Tailwind, Html, Head, Body, pixelBasedPreset } from "@react-email/components";
import { render } from "@react-email/render";
import { viewports } from "./viewport";
import { useContext, useEffect } from "react";
import { TemplateWorkflowContext } from "../../contexts";
import { renderToString } from 'react-dom/server'
import parse from 'html-react-parser';

import { Button, type ButtonProps } from "./components/Button";
import { Container, type ContainerProps } from "./components/Container";
import { Column, type ColumnProps } from "./components/Column";
import { Divider, type DividerProps } from "./components/Divider";
import { Text, type TextProps } from "./components/Text";
import { TextSection, type TextSectionProps } from "./components/TextSection";

import { Pricing, type PricingProps } from "./components/templates/Pricing";
import { PricingEmphasised, type PricingEmphasisedProps } from "./components/templates/PricingEmphasised";

import "@measured/puck/puck.css";
import "./Editor.css";
import { ProjectContext, TemplateContext, CampaignContext } from "@/contexts";
import api from "@/api";

interface Components {
    Button: ButtonProps
    Container: ContainerProps
    Column: ColumnProps
    Divider: DividerProps
    Text: TextProps
    TextSection: TextSectionProps
    Pricing: PricingProps
    PricingEmphasised: PricingEmphasisedProps
}

const config: Config<Components> = {
    categories: {},
    root: {
        fields: {
            title: {
                type: "text",
            },
            preview: {
                type: "textarea",
            },
        },
    },
    components: {
        Button,
        Container,
        Column,
        Divider,
        Text,
        TextSection,
        Pricing,
        PricingEmphasised
    },
}


// The SaveHandler component registers a save handler that is called when the
// user clicks the "Next" button in the CampaignDetail view. This handler has to
// be defined within a separate component to have access to the Puck context.
function SaveHandler() {
    const { onSubmit } = useContext(TemplateWorkflowContext);
    const [project] = useContext(ProjectContext);
    const [campaign] = useContext(CampaignContext);
    const [template, setTemplate] = useContext(TemplateContext);
    const getPuck = useGetPuck();

    onSubmit(async () => {
        const { appState } = getPuck();

        const tailwindConfig = {
            presets: [pixelBasedPreset],
        }

        const content = renderToString(<Render config={config} data={appState.data} />)
        const html = await render(
            <Html lang={template.locale}>
                <Head />
                <Tailwind config={tailwindConfig}>
                    <Body>
                        {parse(content)}
                    </Body>
                </Tailwind>
            </Html>
        );

        const updated = await api.campaigns.templates.update(project.id, campaign.id, template.id, {
            data: {
                ...template.data,
                editor: appState.data,
                html,
            }
        });

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
                iframe: ({ children, document }) => {
                    useEffect(() => {
                        if (document) {
                            const script = document.createElement('script');
                            script.type = 'module';
                            script.src = 'https://cdn.skypack.dev/twind/shim';
                            document.head.appendChild(script);
                        }
                    }, [document]);

                    return <>{children}</>;
                },
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
