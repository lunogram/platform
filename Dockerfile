FROM node:24-alpine AS console
ARG VITE_CLERK_PUBLISHABLE_KEY

WORKDIR /src
RUN corepack enable
COPY console/package.json console/pnpm-lock.yaml ./console/
RUN cd console && pnpm install --frozen-lockfile
COPY console/ ./console/
RUN cd console && pnpm build

FROM golang:1.25-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown

WORKDIR /src
RUN apk add --no-cache git ca-certificates make bash
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=console /src/console/dist ./internal/http/console/dist/
RUN VERSION=${VERSION} SHORT_COMMIT=${COMMIT} make lunogram

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /src/bin/lunogram /app/lunogram
EXPOSE 8080 8081
ENTRYPOINT ["/app/lunogram"]
