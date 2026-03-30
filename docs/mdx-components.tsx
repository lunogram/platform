import defaultMdxComponents from "fumadocs-ui/mdx";
import type { MDXComponents } from "mdx/types";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
  CardAction,
} from "@/components/ui/card";
import { Mermaid } from "@/components/mermaid";
import { EnterpriseFeature, EnterpriseBadge } from "@/components/enterprise";

export function getMDXComponents(components?: MDXComponents): MDXComponents {
  return {
    ...defaultMdxComponents,
    Button,
    Mermaid,
    EnterpriseFeature,
    EnterpriseBadge,
    ShadCard: Card,
    ShadCardHeader: CardHeader,
    ShadCardTitle: CardTitle,
    ShadCardDescription: CardDescription,
    ShadCardContent: CardContent,
    ShadCardFooter: CardFooter,
    ShadCardAction: CardAction,
    ...components,
  };
}
