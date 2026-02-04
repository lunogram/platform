/* eslint-disable react-hooks/rules-of-hooks */
import { Puck, Render, useGetPuck, type Config } from "@puckeditor/core";
import {
  Tailwind,
  Html,
  Head,
  Font,
  Body,
  pixelBasedPreset,
} from "@react-email/components";
import { render } from "@react-email/render";
import { viewports } from "./viewport";
import { useContext, useEffect } from "react";
import { TemplateWorkflowContext } from "../../contexts";
import { renderToString } from "react-dom/server";
import parse from "html-react-parser";

import { Button, type ButtonProps } from "./components/Button";
import { Container, type ContainerProps } from "./components/Container";
import { Column, type ColumnProps } from "./components/Column";
import { Divider, type DividerProps } from "./components/Divider";
import { Text, type TextProps } from "./components/Text";
import { TextSection, type TextSectionProps } from "./components/TextSection";

import { Pricing, type PricingProps } from "./components/templates/Pricing";
import {
  PricingEmphasised,
  type PricingEmphasisedProps,
} from "./components/templates/PricingEmphasised";

import { Img, type ImgProps } from "./components/Image";
import { Link, type LinkProps } from "./components/Link";
import { Heading, type HeadingProps } from "./components/Heading";
import { Markdown, type MarkdownProps } from "./components/Markdown";
import { Section, type SectionProps } from "./components/Section";
import { Row, type RowProps } from "./components/Row";
import { CodeBlock, type CodeBlockProps } from "./components/CodeBlock";
import { CodeInline, type CodeInlineProps } from "./components/CodeInline";

import "@puckeditor/core/dist/index.css";
import "./Editor.css";
import { ProjectContext, TemplateContext, CampaignContext } from "@/contexts";
import api from "@/api";

interface Components {
  Button: ButtonProps;
  Container: ContainerProps;
  Column: ColumnProps;
  Divider: DividerProps;
  Text: TextProps;
  TextSection: TextSectionProps;
  Pricing: PricingProps;
  PricingEmphasised: PricingEmphasisedProps;
  Img: ImgProps;
  Link: LinkProps;
  Heading: HeadingProps;
  Markdown: MarkdownProps;
  Section: SectionProps;
  Row: RowProps;
  CodeBlock: CodeBlockProps;
  CodeInline: CodeInlineProps;
}

const config: Config<Components> = {
  categories: {
    layout: {
      components: ["Container", "Section", "Column", "Divider", "Row"],
    },
    content: {
      components: [
        "Heading",
        "Text",
        "TextSection",
        "Img",
        "Button",
        "Link",
        "Markdown",
        "CodeBlock",
        "CodeInline",
      ],
    },
    templates: { components: ["Pricing", "PricingEmphasised"] },
  },
  root: {
    fields: {
      title: { type: "text" },
      previewText: { type: "text", label: "Inbox Preview" },
      lang: {
        type: "select",
        options: [
          { label: "English", value: "en" },
          { label: "German", value: "de" },
        ],
      },
      fontFamily: { type: "text" },
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
    PricingEmphasised,
    Img,
    Link,
    Heading,
    Markdown,
    Section,
    Row,
    CodeBlock,
    CodeInline,
  },
};

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
    };

    const content = renderToString(
      <Render config={config} data={appState.data} />,
    );
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
          <Body>{parse(content)}</Body>
        </Tailwind>
      </Html>,
    );

    const updated = await api.campaigns.templates.update(
      project.id,
      campaign.id,
      template.id,
      {
        data: {
          ...template.data,
          editor: appState.data,
          html,
        },
      },
    );

    setTemplate(updated);
    return true;
  });

  return null;
}

export default function Editor() {
  const [template] = useContext(TemplateContext);
  const data = template.data.editor ?? { content: [], root: {} };

  return (
    <div className="w-full h-full">
      <Puck
        viewports={viewports}
        config={config}
        data={data}
        overrides={{
          iframe: ({ children, document }) => {
            useEffect(() => {
              if (document) {
                const script = document.createElement("script");
                script.type = "module";
                script.src = "https://cdn.skypack.dev/twind/shim";
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
        }}
      />
    </div>
  );
}
