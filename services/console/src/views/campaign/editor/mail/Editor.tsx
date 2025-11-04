import { Puck, type Config } from "@measured/puck";
import { pixelBasedPreset, Tailwind } from "@react-email/components";

import { Button, type ButtonProps } from "./components/Button";

import "@measured/puck/puck.css";
import { viewports } from "./viewport";

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

export default function Editor() {
    const data = {}

    return (
        <div className="w-full h-full">
            <Puck viewports={viewports} config={config} data={data} />
        </div>
    );
}