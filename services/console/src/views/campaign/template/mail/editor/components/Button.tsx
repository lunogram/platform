import type { ComponentConfig } from '@measured/puck';
import { Button as EmailButton } from '@react-email/components';
import { Decorations, type DecorationsProps } from '../Decorations';
import { Dimensions, type DimensionsProps } from '../Dimensions';

export interface ButtonProps {
    value: string;
    href: string;
    decorations: DecorationsProps;
    dimensions: DimensionsProps;
};

export const Button: ComponentConfig<ButtonProps> = {
    fields: {
        value: {
            type: "text",
            contentEditable: true,
        },
        href: {
            type: "text",
        },
        decorations: Decorations,
        dimensions: Dimensions,
    },
    defaultProps: {
        value: "Next",
        href: "#",
        decorations: {},
        dimensions: {},
    },
    render: ({ value, href, decorations }) => {
        const style = {}

        return (
            <EmailButton
                className="box-border w-full rounded-[8px] bg-indigo-600 text-center font-semibold text-white"
                href={href}
                style={style}
            >
                {value}
            </EmailButton>
        );
    },
}
