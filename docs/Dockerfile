FROM ghcr.io/gofr-dev/website:workflow-package AS base

WORKDIR /app

COPY  quick-start /app/src/app/docs/quick-start
COPY  advanced-guide /app/src/app/docs/advanced-guide
COPY  page.md /app/src/app/docs
COPY  navigation.js /app/src/lib

ENV NODE_ENV production

RUN npm install
RUN npm run build

RUN addgroup -g 1001 -S nodejs
RUN adduser -S nextjs -u 1001

USER nextjs

EXPOSE 3000
ENV PORT 3000

ENV NEXT_TELEMETRY_DISABLED 1

CMD ["node_modules/.bin/next", "start"]