FROM denoland/deno:2.2.6 AS build

WORKDIR /app

# Copy source files
COPY deno.json .
COPY *.ts ./

# Cache dependencies
RUN deno install

FROM denoland/deno:distroless-2.2.6

WORKDIR /app

# Copy cached dependencies and source from the build stage
COPY --from=build --chown=1993:1993 /app .
COPY --from=build --chown=1993:1993 /deno-dir/ /deno-dir/

USER 1993

# Run with minimal permissions. Beyond network and env, mjml needs homedir and
# a readable working directory: it resolves a config path via env-paths on
# import and stats its `filePath` option before parsing.
#
# env-paths also calls node:os tmpdir(), which tries TMPDIR, TMP and TEMP in
# turn. Deno implements that through Deno.env.get, so an unlisted variable
# raises NotCapable rather than returning undefined and the fallback chain dies
# on the first one it touches. All three must be listed even though the image
# sets none of them and tmpdir() ends up at its /tmp default.
CMD ["run", "--allow-net", "--allow-env=NATS_URL,TEMPLATICAL_SOCIAL_ICONS_BASE_URL,TMPDIR,TMP,TEMP", "--allow-sys=homedir", "--allow-read=/app", "main.ts"]
