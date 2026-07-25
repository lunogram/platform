import { renderToMjml } from "@templatical/renderer";
import type { TemplateContent } from "@templatical/types";
import mjml2html from "mjml";
import { convert } from "html-to-text";

/**
 * Base URL for the social icon PNGs embedded in rendered emails.
 *
 * Templatical defaults to a jsDelivr URL, which would make every sent email
 * depend on a third-party CDN. Set TEMPLATICAL_SOCIAL_ICONS_BASE_URL to serve
 * the assets from the platform instead. When unset, the library default
 * applies.
 */
const SOCIAL_ICONS_BASE_URL = Deno.env.get(
  "TEMPLATICAL_SOCIAL_ICONS_BASE_URL",
);

/**
 * Narrow an arbitrary parsed JSON value to a Templatical document.
 *
 * A document is `{ blocks: Block[], settings: TemplateSettings }`. React Email
 * templates arrive as JSX source, which is not valid JSON at all, so this
 * check only ever runs against something that already parsed cleanly.
 */
export function isTemplaticalDocument(
  value: unknown,
): value is TemplateContent {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as Partial<TemplateContent>;
  return (
    Array.isArray(candidate.blocks) &&
    typeof candidate.settings === "object" &&
    candidate.settings !== null
  );
}

/**
 * Render a Templatical document to HTML and plain text.
 *
 * Unlike the React Email path this takes no props: merge tags survive the
 * render as literal `{{ … }}` and are substituted downstream by the platform's
 * Liquid pass. Being prop-independent means the output can be computed once at
 * compile time and reused for every recipient.
 */
export async function renderTemplaticalDocument(
  doc: TemplateContent,
): Promise<{ html: string; plainText: string }> {
  const mjmlSource = await renderToMjml(
    doc,
    SOCIAL_ICONS_BASE_URL
      ? { socialIconsBaseUrl: SOCIAL_ICONS_BASE_URL }
      : undefined,
  );

  const { html, errors } = await mjml2html(mjmlSource, {
    validationLevel: "soft",
  });

  // Soft validation still produces usable HTML, so these are warnings rather
  // than failures — log them instead of failing a send.
  if (errors?.length) {
    console.warn(
      `mjml reported ${errors.length} validation issue(s):`,
      errors.map((e: { formattedMessage?: string }) => e.formattedMessage)
        .join("; "),
    );
  }

  return { html, plainText: toPlainText(html) };
}

/**
 * Derive the plain-text alternative from rendered HTML.
 *
 * Images are dropped rather than rendered as alt text, and link URLs are kept
 * inline so the text part stays actionable.
 *
 * Headings must opt out of html-to-text's default uppercasing: it would turn a
 * merge tag inside a heading into `{{ USER.FIRST_NAME }}`, which the platform's
 * Liquid pass cannot resolve, silently leaving the raw tag in the text part of
 * every email.
 */
function toPlainText(html: string): string {
  return convert(html, {
    wordwrap: 80,
    selectors: [
      { selector: "img", format: "skip" },
      { selector: "a", options: { hideLinkHrefIfSameAsText: true } },
      ...["h1", "h2", "h3", "h4", "h5", "h6"].map((selector) => ({
        selector,
        options: { uppercase: false },
      })),
    ],
  });
}
