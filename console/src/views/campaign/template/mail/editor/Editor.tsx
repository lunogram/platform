/* eslint-disable react-hooks/rules-of-hooks */
import { Puck, Render, useGetPuck, type Config } from "@measured/puck";
import { Tailwind, Html, Head, Font, Body, pixelBasedPreset } from "@react-email/components";
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
import { oapiClient } from "@/oapi/client";

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
        console.log(appState)

        const tailwindConfig = {
            presets: [pixelBasedPreset],
        }

        const content = renderToString(<Render config={config} data={appState.data} />)
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
                    <Body>
                        {parse(content)}
                    </Body>
                </Tailwind>
            </Html>
        );

        const updated = await oapiClient.PATCH("/api/admin/projects/{projectID}/campaigns/{campaignID}/templates/{templateID}", {
            params: {
                path: {
                    projectID: project.id,
                    campaignID: campaign.id,
                    templateID: template.id,
                }
            },
            body: {
                data: {
                    editor: appState.data,
                    html: html,
                },
            }
        });

        if (!updated.data) {
            return false;
        }
        
        setTemplate(updated.data);
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
