# --------------> The compiler image
FROM node:22 AS compile
WORKDIR /usr/src/app
RUN corepack enable
COPY . .
RUN pnpm install --filter "@parcelvoy/platform" --frozen-lockfile --config.platform=linux --config.architecture=x64
RUN pnpm --filter "@parcelvoy/platform" build
RUN pnpm --filter "@parcelvoy/platform" --prod deploy packages

# --------------> The production image
FROM node:22-alpine
RUN apk add dumb-init
ENV NODE_ENV="production"
USER node
WORKDIR /usr/src/app
COPY --chown=node:node --from=compile /usr/src/app/packages/node_modules ./node_modules
COPY --chown=node:node --from=compile /usr/src/app/apps/platform/package*.json ./
COPY --chown=node:node --from=compile /usr/src/app/apps/platform/build ./build
COPY --chown=node:node --from=compile /usr/src/app/apps/platform/db ./db
COPY --chown=node:node --from=compile /usr/src/app/apps/platform/public ./public
EXPOSE 3001
CMD ["dumb-init", "node", "build/boot.js"]
