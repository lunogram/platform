import { createUsePuck, type AsFieldProps, type Field } from "@measured/puck";
import { Scan } from "lucide-react";

import CollapsibleField from "./CollapsibleField";
import { Button } from "@/components/ui/button";

export interface DecorationsViewport {
    borderRadius: { t: number; r: number; b: number; l: number };
    boxShadow: string;
    backgroundColor: string;
}

export interface DecorationsProps {
    sm?: Partial<DecorationsViewport>;
    md?: Partial<DecorationsViewport>;
    xl?: Partial<DecorationsViewport>;
}

const usePuck = createUsePuck();

export const Decorations: Field<AsFieldProps<unknown>, DecorationsProps> = {
    type: "custom",
    render: ({ onChange, value }) => {
        const viewport = usePuck((s) => s.appState.ui.viewports.current);
        // TODO: set values for current viewport

        return (
            <CollapsibleField icon={<Scan />} title="Decorations">
                <div className="grid grid-cols-2 gap-2">
                    <Button>1</Button>
                    <Button>2</Button>
                    <Button>3</Button>
                    <Button>4</Button>
                </div>
            </CollapsibleField>
        )
    }
}