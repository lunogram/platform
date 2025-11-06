import { createUsePuck, type AsFieldProps, type Field } from "@measured/puck";
import { VectorSquare } from "lucide-react";

import CollapsibleField from "./CollapsibleField";
import { InputWithUnit } from "./InputWithUnit";

export interface DimensionsViewport {
    borderRadius: { t: number; r: number; b: number; l: number };
    boxShadow: string;
    backgroundColor: string;
}

export interface DimensionsProps {
    sm?: Partial<DimensionsViewport>;
    md?: Partial<DimensionsViewport>;
    xl?: Partial<DimensionsViewport>;
}

const usePuck = createUsePuck();

export const Dimensions: Field<AsFieldProps<unknown>, DimensionsProps> = {
    type: "custom",
    render: ({ onChange, value }) => {
        const viewport = usePuck((s) => s.appState.ui.viewports.current);
        return (
            <CollapsibleField icon={<VectorSquare />} title="Dimensions">
                <div className="grid grid-cols-2 gap-2">
                    <InputWithUnit />
                    <InputWithUnit />
                </div>
                <div className="grid grid-cols-2 gap-2">
                    <InputWithUnit />
                    <InputWithUnit />
                </div>
                <div className="grid grid-cols-2 gap-2">
                    <InputWithUnit />
                    <InputWithUnit />
                    <InputWithUnit />
                    <InputWithUnit />
                </div>
                <div className="grid grid-cols-2 gap-2">
                    <InputWithUnit />
                    <InputWithUnit />
                    <InputWithUnit />
                    <InputWithUnit />
                </div>
            </CollapsibleField>
        )
    }
}
