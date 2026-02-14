import defaultMdxComponents from 'fumadocs-ui/mdx';
import type { MDXComponents } from 'mdx/types';
import { Button } from '@/components/ui/button';
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter, CardAction } from '@/components/ui/card';

export function getMDXComponents(components?: MDXComponents): MDXComponents {
  return {
    ...defaultMdxComponents,
    Button,
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
