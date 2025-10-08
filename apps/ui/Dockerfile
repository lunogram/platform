# --------------> The compiler image
FROM node:22 AS compile
WORKDIR /usr/src/app
RUN corepack enable
COPY . .
RUN corepack enable
WORKDIR /usr/src/app/apps/ui
RUN pnpm install --frozen-lockfile
RUN pnpm build

# --------------> The production image
FROM nginx:1.23.4-alpine
EXPOSE 3000
COPY --from=compile /usr/src/app/apps/ui/docker/nginx/conf.d/default.conf /etc/nginx/conf.d/default.conf
COPY --from=compile /usr/src/app/apps/ui/build /usr/share/nginx/html
COPY --from=compile /usr/src/app/apps/ui/scripts /usr/share/nginx/html
WORKDIR /usr/share/nginx/html
RUN apk add --no-cache bash
RUN chmod +x env.sh
CMD ["/bin/bash", "-c", "/usr/share/nginx/html/env.sh && nginx -g \"daemon off;\""]