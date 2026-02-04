import { Img as EmailImg } from "@react-email/components";
import { type ComponentConfig } from "@puckeditor/core";
import { cn } from "@/utils";
import { Layout, layoutClassMap, type LayoutProps } from "./fields/Layout";
import { Spacing, spacingClassMap, type SpacingProps } from "./fields/Spacing";
import { generateTailwindClasses } from "./fields/unit";
import { ImageUpload, type ImageUploadProps } from "./fields/ImageUpload";

export interface ImgProps {
  src: string;
  alt: string;
  width?: string;
  height?: string;
  image: ImageUploadProps;
  layout: LayoutProps;
  spacing: SpacingProps;
}

export const Img: ComponentConfig<ImgProps> = {
  fields: {
    src: { type: "text", label: "Or paste URL" },
    alt: { type: "text" },
    width: { type: "text" },
    height: { type: "text" },
    image: ImageUpload,
    layout: Layout,
    spacing: Spacing,
  },
  defaultProps: {
    src: "https://upload.wikimedia.org/wikipedia/commons/b/b6/Image_created_with_a_mobile_phone.png",
    alt: "Image",
    image: {},
    layout: {},
    spacing: {},
  },
  render: ({ src, alt, width, height, image, layout, spacing }) => {
    const classes = cn(
      generateTailwindClasses(layout, layoutClassMap),
      generateTailwindClasses(spacing, spacingClassMap),
    );

    const imageUrl = 
      image?.xl?.url || 
      image?.md?.url || 
      image?.sm?.url || 
      src;
    
    return (
      <EmailImg
        src={imageUrl}
        alt={alt}
        width={width}
        height={height}
        className={classes}
      />
    );
  },
};