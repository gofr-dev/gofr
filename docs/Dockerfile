FROM ghcr.io/gofr-dev/website:workflow-package AS base

WORKDIR /app

COPY  v1/ /app/src/app/docs
COPY  page.md /app/src/app/docs
COPY  navigation.js /app/src/lib

ENV NODE_ENV production

EXPOSE 3000
ENV PORT 3000
ENV NEXT_TELEMETRY_DISABLED 1

CMD ["node_modules/.bin/next", "start"]