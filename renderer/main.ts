import { connect } from "@nats-io/transport-deno";
import type { NatsConnection, Msg } from "@nats-io/nats-core";
import { compile, renderTemplate } from "./compiler.ts";

/** Request payload for email.compile.{project_id} */
interface CompileRequest {
  source: string;
}

/** Response payload for email.compile */
interface CompileResponse {
  compiled_js: string;
  error?: string;
}

/** Request payload for email.render.{project_id} */
interface RenderRequest {
  compiled_js: string;
  props: Record<string, unknown>;
}

/** Response payload for email.render */
interface RenderResponse {
  html: string;
  plain_text: string;
  error?: string;
}

const NATS_URL = Deno.env.get("NATS_URL") ?? "nats://localhost:4222";

function encodeJSON(data: unknown): Uint8Array {
  return new TextEncoder().encode(JSON.stringify(data));
}

async function handleCompile(msg: Msg): Promise<void> {
  try {
    const request: CompileRequest = JSON.parse(
      new TextDecoder().decode(msg.data),
    );

    const compiledJs = await compile(request.source);

    const response: CompileResponse = { compiled_js: compiledJs };
    msg.respond(encodeJSON(response));
  } catch (err) {
    const response: CompileResponse = {
      compiled_js: "",
      error: err instanceof Error ? err.message : String(err),
    };
    msg.respond(encodeJSON(response));
  }
}

async function handleRender(msg: Msg): Promise<void> {
  try {
    const request: RenderRequest = JSON.parse(
      new TextDecoder().decode(msg.data),
    );

    const result = await renderTemplate(request.compiled_js, request.props);

    const response: RenderResponse = {
      html: result.html,
      plain_text: result.plainText,
    };
    msg.respond(encodeJSON(response));
  } catch (err) {
    const response: RenderResponse = {
      html: "",
      plain_text: "",
      error: err instanceof Error ? err.message : String(err),
    };
    msg.respond(encodeJSON(response));
  }
}

async function main(): Promise<void> {
  console.log(`connecting to NATS at ${NATS_URL}`);

  const nc: NatsConnection = await connect({ servers: NATS_URL });
  console.log("connected to NATS");

  // Subscribe to compile requests (email.compile.>)
  const compileSub = nc.subscribe("email.compile.>");
  console.log("listening on email.compile.>");

  // Subscribe to render requests (email.render.>)
  const renderSub = nc.subscribe("email.render.>");
  console.log("listening on email.render.>");

  // Process compile requests
  (async () => {
    for await (const msg of compileSub) {
      handleCompile(msg).catch((err) =>
        console.error("unhandled compile error:", err),
      );
    }
  })();

  // Process render requests
  (async () => {
    for await (const msg of renderSub) {
      handleRender(msg).catch((err) =>
        console.error("unhandled render error:", err),
      );
    }
  })();

  // Wait for NATS connection to close
  await nc.closed();
  console.log("NATS connection closed");
}

main().catch((err) => {
  console.error("fatal error:", err);
  Deno.exit(1);
});
