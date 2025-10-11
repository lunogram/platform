FROM node:24 AS build
WORKDIR /usr/src/app
RUN corepack enable
COPY . .
RUN corepack enable
WORKDIR /usr/src/app/apps/ui
RUN pnpm install --frozen-lockfile
RUN pnpm build

FROM nginx:1.29-alpine

COPY --from=build /usr/src/app/apps/ui/docker/nginx/conf.d/default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /usr/src/app/apps/ui/build /usr/share/nginx/html
COPY --from=build /usr/src/app/apps/ui/scripts /usr/share/nginx/html

EXPOSE 8080

WORKDIR /usr/share/nginx/html

# TODO: we have to consider changing our approach here and eithe use a envsubst approach or build a image per environment
RUN apk add --no-cache bash
RUN chmod +x env.sh

CMD ["/bin/bash", "-c", "/usr/share/nginx/html/env.sh && nginx -g \"daemon off;\""]