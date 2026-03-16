export const DEFAULT_REACT_EMAIL_TEMPLATE = `import {
  Body,
  Container,
  Head,
  Heading,
  Hr,
  Html,
  Img,
  Link,
  Preview,
  Section,
  Text,
  Button,
  Font,
  Row,
  Column,
  Tailwind,
} from "@react-email/components";

const baseUrl = "https://console.lunogram.com";
const docsUrl = "https://docs.lunogram.com";

export default function Email(props: EmailProps) {
  return (
    <Html lang="en">
      <Head>
        <Font
          fontFamily="Inter"
          fallbackFontFamily="Helvetica"
          webFont={{
            url: "https://fonts.gstatic.com/s/inter/v18/UcCO3FwrK3iLTeHuS_nVMrMxCp50SjIw2boKoduKmMEVuLyfAZ9hiA.woff2",
            format: "woff2",
          }}
          fontWeight={400}
          fontStyle="normal"
        />
        <Font
          fontFamily="Inter"
          fallbackFontFamily="Helvetica"
          webFont={{
            url: "https://fonts.gstatic.com/s/inter/v18/UcCO3FwrK3iLTeHuS_nVMrMxCp50SjIw2boKoduKmMEVuGKYAZ9hiA.woff2",
            format: "woff2",
          }}
          fontWeight={600}
          fontStyle="normal"
        />
      </Head>
      <Preview>
        Import users, create journeys, and start engaging your customers.
      </Preview>
      <Tailwind>
        <Body className="bg-[#fafafa] font-sans antialiased">
          <Container className="max-w-140 mx-auto px-4 py-10">
            {/* Logo */}
            <Section className="text-center pb-8">
              <Img
                src="https://lunogram.com/logos/logo-black-512.png"
                width="140"
                alt="Lunogram"
                className="mx-auto"
              />
            </Section>

            {/* Main card */}
            <Section className="bg-white border border-zinc-200 rounded-xl overflow-hidden">
              {/* Header */}
              <Section className="px-10 pt-10">
                <table role="presentation" cellPadding={0} cellSpacing={0}>
                  <tr>
                    <td className="bg-green-50 border border-green-200 rounded-full px-3 py-1">
                      <span className="text-xs font-medium text-green-600 tracking-wide">
                        Project Created
                      </span>
                    </td>
                  </tr>
                </table>
                <Heading
                  as="h1"
                  className="m-0 mt-5 text-2xl font-semibold text-zinc-950 leading-snug tracking-tight"
                >
                  Your project is ready!
                </Heading>
                <Text className="m-0 mt-2 mb-8 text-[15px] leading-relaxed text-zinc-500">
                  Your project has been created. Here's how to get the most out
                  of Lunogram from day one.
                </Text>
              </Section>

              <Hr className="border-t border-zinc-200 border-b-0 border-l-0 border-r-0 mx-10" />

              {/* Step 1: Import Users */}
              <Section className="px-10 pt-8">
                <Row>
                  <Column className="w-12 align-top pr-4">
                    <div className="w-10 h-10 bg-zinc-100 border border-zinc-200 rounded-[10px] text-center leading-10">
                      <span className="text-base font-semibold text-zinc-950">
                        1
                      </span>
                    </div>
                  </Column>
                  <Column>
                    <Heading
                      as="h2"
                      className="m-0 mb-1.5 text-base font-semibold text-zinc-950 leading-snug"
                    >
                      Import Users &amp; Organizations
                    </Heading>
                    <Text className="m-0 mb-4 text-sm leading-relaxed text-zinc-500">
                      Sync your users and organizations via the API. This
                      unlocks segmentation, personalized messaging, and
                      organization-level targeting for B2B workflows.
                    </Text>

                    {/* Code block */}
                    <Section className="bg-zinc-900 rounded-lg px-5 py-4">
                      <code
                        style={{
                          fontFamily:
                            "'SF Mono', SFMono-Regular, ui-monospace, Menlo, Monaco, Consolas, monospace",
                          fontSize: "13px",
                          lineHeight: "1.7",
                          color: "#a1a1aa",
                        }}
                      >
                        <span style={{ color: "#a78bfa" }}>POST</span>{" "}
                        <span style={{ color: "#e4e4e7" }}>/v1/users</span>
                        <br />
                        <span style={{ color: "#71717a" }}>{"{"}</span>
                        <br />
                        {"  "}
                        <span style={{ color: "#7dd3fc" }}>"name"</span>
                        <span style={{ color: "#71717a" }}>:</span>{" "}
                        <span style={{ color: "#86efac" }}>"Jane Doe"</span>
                        <span style={{ color: "#71717a" }}>,</span>
                        <br />
                        {"  "}
                        <span style={{ color: "#7dd3fc" }}>"email"</span>
                        <span style={{ color: "#71717a" }}>:</span>{" "}
                        <span style={{ color: "#86efac" }}>"jane@acme.co"</span>
                        <span style={{ color: "#71717a" }}>,</span>
                        <br />
                        {"  "}
                        <span style={{ color: "#7dd3fc" }}>
                          "organization_id"
                        </span>
                        <span style={{ color: "#71717a" }}>:</span>{" "}
                        <span style={{ color: "#86efac" }}>"org_acme"</span>
                        <br />
                        <span style={{ color: "#71717a" }}>{"}"}</span>
                      </code>
                    </Section>

                    <Link
                      href={\`\${docsUrl}/api/users\`}
                      className="inline-block text-[13px] font-medium text-violet-700 no-underline mt-4"
                    >
                      View API docs →
                    </Link>
                  </Column>
                </Row>
              </Section>

              {/* Step 2: Journeys */}
              <Section className="px-10 pt-8">
                <Row>
                  <Column className="w-12 align-top pr-4">
                    <div className="w-10 h-10 bg-zinc-100 border border-zinc-200 rounded-[10px] text-center leading-10">
                      <span className="text-base font-semibold text-zinc-950">
                        2
                      </span>
                    </div>
                  </Column>
                  <Column>
                    <Heading
                      as="h2"
                      className="m-0 mb-1.5 text-base font-semibold text-zinc-950 leading-snug"
                    >
                      Create Journey Experiences
                    </Heading>
                    <Text className="m-0 mb-4 text-sm leading-relaxed text-zinc-500">
                      Design automated, multi-step journeys that guide users
                      through onboarding, re-engagement, and lifecycle stages.
                      Use the visual builder or the API to craft immersive
                      flows.
                    </Text>

                    {/* Journey flow diagram */}
                    <Section className="bg-[#fafafa] border border-zinc-200 rounded-lg px-6 py-5">
                      <table
                        role="presentation"
                        cellPadding={0}
                        cellSpacing={0}
                        className="mx-auto"
                      >
                        <tr>
                          <td className="text-center align-middle">
                            <div className="inline-block bg-white border border-zinc-200 rounded-lg px-3.5 py-2">
                              <span className="text-xs font-medium text-zinc-950">
                                Entrance
                              </span>
                            </div>
                          </td>
                          <td className="px-1.5 align-middle">
                            <div className="w-6 h-0.5 bg-zinc-300" />
                          </td>
                          <td className="text-center align-middle">
                            <div className="inline-block bg-yellow-50 border border-amber-200 rounded-lg px-3.5 py-2">
                              <span className="text-xs font-medium text-amber-700">
                                Delay
                              </span>
                            </div>
                          </td>
                          <td className="px-1.5 align-middle">
                            <div className="w-6 h-0.5 bg-zinc-300" />
                          </td>
                          <td className="text-center align-middle">
                            <div className="inline-block bg-violet-50 border border-violet-300 rounded-lg px-3.5 py-2">
                              <span className="text-xs font-medium text-violet-700">
                                Email
                              </span>
                            </div>
                          </td>
                        </tr>
                      </table>
                    </Section>

                    <Link
                      href={\`\${docsUrl}/journeys\`}
                      className="inline-block text-[13px] font-medium text-violet-700 no-underline mt-4"
                    >
                      Learn about journeys →
                    </Link>
                  </Column>
                </Row>
              </Section>

              {/* CTA */}
              <Section className="px-10 pt-9 pb-10 text-center">
                <Button
                  href={baseUrl}
                  className="inline-block bg-zinc-950 text-white text-sm font-medium no-underline px-8 py-3 rounded-lg tracking-tight"
                >
                  Open your project
                </Button>
              </Section>
            </Section>

            {/* Footer */}
            <Section className="pt-8 text-center">
              <Text className="m-0 mb-2 text-[13px] text-zinc-400 leading-relaxed">
                <Link
                  href={docsUrl}
                  className="text-zinc-500 no-underline"
                >
                  Docs
                </Link>
                {"  ·  "}
                <Link
                  href="https://github.com/lunogram/platform"
                  className="text-zinc-500 no-underline"
                >
                  GitHub
                </Link>
                {"  ·  "}
                <Link
                  href="https://lunogram.com"
                  className="text-zinc-500 no-underline"
                >
                  Website
                </Link>
              </Text>
              <Text className="m-0 text-xs text-zinc-300 leading-relaxed">
                Lunogram, Customer engagement, simplified.
              </Text>
            </Section>
          </Container>
        </Body>
      </Tailwind>
    </Html>
  );
}
`
