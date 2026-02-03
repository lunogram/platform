import { Img as EmailImg } from "@react-email/components";
import { type ComponentConfig } from "@puckeditor/core";
import { cn } from "@/utils";
import { Layout, layoutClassMap, type LayoutProps } from "./fields/Layout";
import { Spacing, spacingClassMap, type SpacingProps } from "./fields/Spacing";
import { generateTailwindClasses } from "./fields/unit";

export interface ImgProps {
  src: string;
  alt: string;
  width?: string;
  height?: string;
  layout: LayoutProps;
  spacing: SpacingProps;
}

export const Img: ComponentConfig<ImgProps> = {
  fields: {
    src: { type: "text" },
    alt: { type: "text" },
    width: { type: "text" },
    height: { type: "text" },
    layout: Layout,
    spacing: Spacing,
  },
  defaultProps: {
    src: "https://upload.wikimedia.org/wikipedia/commons/b/b6/Image_created_with_a_mobile_phone.png",
    alt: "Image",
    layout: {},
    spacing: {},
  },
  render: ({ src, alt, width, height, layout, spacing }) => {
    const classes = cn(
      generateTailwindClasses(layout, layoutClassMap),
      generateTailwindClasses(spacing, spacingClassMap),
    );
    return <EmailImg src={src} alt={alt} width={width} height={height} className={classes} />;
  },
};