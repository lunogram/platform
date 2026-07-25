import {
  assert,
  assertEquals,
  assertRejects,
  assertStringIncludes,
} from "@std/assert";
import {
  createButtonBlock,
  createDefaultTemplateContent,
  createImageBlock,
  createParagraphBlock,
  createSectionBlock,
  createTitleBlock,
} from "@templatical/types";
import { compile, renderTemplate } from "./compiler.ts";

/** A Templatical document with two columns, an image and merge tags. */
function templaticalFixture() {
  const content = createDefaultTemplateContent();
  content.blocks = [
    createSectionBlock({
      columns: "2",
      children: [
        [
          createImageBlock({
            src: "https://cdn.example.com/hero.png",
            alt: "Hero",
            width: "full",
            align: "center",
          }),
        ],
        [
          createTitleBlock({
            content: "Hello {{ user.first_name }}",
            level: 2,
          }),
          createParagraphBlock({ content: "<p>Thanks for joining.</p>" }),
          createButtonBlock({
            text: "Open",
            url: "https://example.com/{{ user.id }}",
          }),
        ],
      ],
    }),
  ];
  return content;
}

const REACT_EMAIL_FIXTURE = `
import { Html, Text } from "@react-email/components";

export default function Email(props: { name: string }) {
  return (
    <Html>
      <Text>Hello {props.name}</Text>
    </Html>
  );
}
`;

Deno.test("templatical: compile produces a kind-tagged bundle", async () => {
  const bundle = JSON.parse(
    await compile(JSON.stringify(templaticalFixture())),
  );

  assertEquals(bundle.kind, "templatical");
  assert(typeof bundle.html === "string" && bundle.html.length > 0);
  assert(typeof bundle.plainText === "string");
  // Rendering happens at compile time, so no document is carried forward.
  assertEquals(bundle.doc, undefined);
});

Deno.test("templatical: renders blocks, images and merge tags", async () => {
  const bundle = await compile(JSON.stringify(templaticalFixture()));
  const { html, plainText } = await renderTemplate(bundle, {});

  assertStringIncludes(html, "https://cdn.example.com/hero.png");
  assertStringIncludes(html, "{{ user.first_name }}");
  assertStringIncludes(html, "{{ user.id }}");
  // MJML output must carry the responsive and Outlook scaffolding.
  assertStringIncludes(html, "@media");
  assertStringIncludes(html, "<!--[if mso");
  // Plain text keeps merge tags and drops images.
  assertStringIncludes(plainText, "{{ user.first_name }}");
  assert(!plainText.includes("hero.png"));
});

Deno.test("templatical: render ignores props", async () => {
  const bundle = await compile(JSON.stringify(templaticalFixture()));

  const a = await renderTemplate(bundle, {});
  const b = await renderTemplate(bundle, { user: { first_name: "Ada" } });

  assertEquals(a.html, b.html);
});

Deno.test("react-email: bundle stays untagged and transpiles", async () => {
  const parsed = JSON.parse(await compile(REACT_EMAIL_FIXTURE));

  // No `kind` — its absence is what routes a bundle to the JSX path, and every
  // bundle stored before this branch existed lacks it.
  assertEquals(parsed.kind, undefined);
  assertStringIncludes(parsed.code, "__Component__");
  // Imports are stripped; the scope is injected at render time instead.
  assert(!parsed.code.includes("import "));

  // Executing the bundle is deliberately not asserted here. `deno test` pulls
  // react and react-dom into one module graph, where a pre-existing version
  // skew in deno.json (react 18.3.1 against react-dom 19.x, via
  // @react-email/render's caret range) trips React's isomorphic version check.
  // `deno run` — how the service actually starts — is unaffected.
});

Deno.test("react-email: JSON that is not a document falls through to JSX", async () => {
  // Valid JSON, but not a Templatical document. It must not be misrouted into
  // the MJML path — it goes to Sucrase, which rejects it as JSX. The rejection
  // is the evidence of correct routing.
  await assertRejects(() => compile('{"blocks": "not-an-array"}'), SyntaxError);
});
