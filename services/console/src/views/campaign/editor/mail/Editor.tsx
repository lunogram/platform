import { Puck, Render, useGetPuck, type Config } from "@measured/puck";
import { pixelBasedPreset, Tailwind, Html, Head, Body } from "@react-email/components";
import { render, pretty } from "@react-email/render";
import { viewports } from "./viewport";
import { useContext } from "react";
import { CampaignDetailContext } from "../../contexts";

import { Button, type ButtonProps } from "./components/Button";

import "@measured/puck/puck.css";
import "./Editor.css";

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
                <Html lang="en">
                    <Head />
                    <Tailwind config={config}>
                        <Body>
                            {children}
                        </Body>
                    </Tailwind>
                </Html>
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
    const { onNext } = useContext(CampaignDetailContext);
    const getPuck = useGetPuck();

    onNext(async () => {
        const { appState } = getPuck();
        const html = await pretty(await render(<Render config={config} data={appState.data} />));
        console.log(html)

        await new Promise(resolve => setTimeout(resolve, 1000));
    });

    return null;
}

export default function Editor() {
    const data = {}

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
