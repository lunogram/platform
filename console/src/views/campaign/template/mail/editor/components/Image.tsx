import { Img as EmailImg } from "@react-email/components";
import { type ComponentConfig } from "@puckeditor/core";
import { cn } from "@/utils";
import { Layout, layoutClassMap, type LayoutProps } from "./fields/Layout";
import { Spacing, spacingClassMap, type SpacingProps } from "./fields/Spacing";
import { Decoration, decorationClassMap, type DecorationProps } from "./fields/Decoration";
import { generateTailwindClasses } from "./fields/unit";

export interface ImgProps {
  src: string;
  alt: string;
  width?: string;
  height?: string;
  layout: LayoutProps;
  spacing: SpacingProps;
  decoration: DecorationProps;
}

export const Img: ComponentConfig<ImgProps> = {
  fields: {
    src: { type: "text" },
    alt: { type: "text" },
    width: { type: "text" },
    height: { type: "text" },
    layout: Layout,
    spacing: Spacing,
    decoration: Decoration,
  },
  defaultProps: {
    src: "https://upload.wikimedia.org/wikipedia/commons/b/b6/Image_created_with_a_mobile_phone.png",
    alt: "Image",
    layout: {},
    spacing: {},
    decoration: {},
  },
  render: ({ src, alt, width, height, layout, spacing, decoration }) => {
    const classes = cn(
      generateTailwindClasses(layout, layoutClassMap),
      generateTailwindClasses(spacing, spacingClassMap),
      generateTailwindClasses(decoration, decorationClassMap)
    );
    return <EmailImg src={src} alt={alt} width={width} height={height} className={classes} />;
  },
};