import { type Config } from "@puckeditor/core";

import { Button, type ButtonProps } from "../components/Button";
import { Container, type ContainerProps } from "../components/Container";
import { Column, type ColumnProps } from "../components/Column";
import { Divider, type DividerProps } from "../components/Divider";
import { Text, type TextProps } from "../components/Text";
import { TextSection, type TextSectionProps } from "../components/TextSection";

import { Pricing, type PricingProps } from "../components/templates/Pricing";
import {
  PricingEmphasised,
  type PricingEmphasisedProps,
} from "../components/templates/PicingEmphasised/PricingEmphasised";

import { Img, type ImgProps } from "../components/Image";
import { Link, type LinkProps } from "../components/Link";
import { Heading, type HeadingProps } from "../components/Heading";
import { Markdown, type MarkdownProps } from "../components/Markdown";
import { Section, type SectionProps } from "../components/Section";
import { Row, type RowProps } from "../components/Row";
import { CodeBlock, type CodeBlockProps } from "../components/CodeBlock";
import { CodeInline, type CodeInlineProps } from "../components/CodeInline";

export interface Components {
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

export const config: Config<Components> = {
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
    PricingEmphasised,
    Button,
    Container,
    Column,
    Divider,
    Text,
    TextSection,
    Pricing,
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
