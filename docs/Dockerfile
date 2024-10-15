FROM ghcr.io/gofr-dev/website:latest

WORKDIR /app

COPY  docs/quick-start /app/src/app/docs/quick-start
COPY  docs/public/ /app/public
COPY  docs/advanced-guide /app/src/app/docs/advanced-guide
COPY  docs/references /app/src/app/docs/references
COPY  docs/page.md /app/src/app/docs
COPY  docs/navigation.js /app/src/lib


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