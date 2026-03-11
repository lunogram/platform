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

# Run with minimal permissions: only network access to NATS and env vars
CMD ["run", "--allow-net", "--allow-env=NATS_URL", "main.ts"]
