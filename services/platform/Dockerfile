FROM node:24 AS build

WORKDIR /usr/src/app

RUN corepack enable

COPY . .

RUN pnpm install --filter "@lunogram/platform" --frozen-lockfile --config.platform=linux --config.architecture=x64
RUN pnpm --filter "@lunogram/platform" build
RUN pnpm --filter "@lunogram/platform" --prod deploy packages

FROM gcr.io/distroless/nodejs24-debian12:nonroot
ENV NODE_ENV=production
WORKDIR /usr/src/app

COPY --chown=65532:65532 --from=build /usr/src/app/packages/node_modules ./node_modules
COPY --chown=65532:65532 --from=build /usr/src/app/services/platform/package*.json ./
COPY --chown=65532:65532 --from=build /usr/src/app/services/platform/build ./build
COPY --chown=65532:65532 --from=build /usr/src/app/services/platform/db ./build/db

EXPOSE 8080

CMD ["build/src/boot.js"]