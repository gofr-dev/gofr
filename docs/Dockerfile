# WEBSITE_TAG selects which published source-bundle of the website
# repo to overlay docs onto. The two CI flows pass it explicitly:
#   - .github/workflows/website-stage.yml → WEBSITE_TAG=stage
#       (built from main of gofr-dev/website on every merge)
#   - .github/workflows/website-prod.yml  → WEBSITE_TAG=latest
#       (built from a tagged release of gofr-dev/website)
# Default is `latest` so a hand-run `docker build -f docs/Dockerfile`
# without any --build-arg still produces a prod-flavored image.
ARG WEBSITE_TAG=latest
FROM ghcr.io/gofr-dev/website:${WEBSITE_TAG} AS builder

WORKDIR /app

COPY  docs/quick-start /app/src/app/docs/quick-start
COPY  docs/public/ /app/public
COPY  docs/advanced-guide /app/src/app/docs/advanced-guide
COPY  docs/guides         /app/src/app/docs/guides
COPY docs/datasources /app/src/app/docs/datasources
COPY  docs/references /app/src/app/docs/references
COPY  docs/page.md /app/src/app/docs
COPY  docs/navigation.js /app/src/lib
COPY  docs/events.json /app/src/app/events
COPY  docs/testimonials.json /app/utils

# Framework-content marketing pages. Live in this repo (not the website
# repo) so maintainers can edit copy alongside framework changes in a
# single PR. Layouts for these routes stay in the website — only the
# .md content overlays here.
COPY  docs/why-gofr     /app/src/app/why-gofr
COPY  docs/comparison   /app/src/app/comparison
COPY  docs/migrate      /app/src/app/migrate
COPY  docs/learn        /app/src/app/learn
COPY  docs/faq          /app/src/app/faq

# Framework-knowledge data files. Same logic as events.json /
# testimonials.json above — content describing the framework lives
# with the framework.
COPY  docs/team.json    /app/src/data/team.json
COPY  docs/showcase/companies.json  /app/src/app/showcase/companies.json
COPY  docs/examples/examples.json   /app/src/app/examples/examples.json

# AGENTS.md — curated rules for AI coding assistants. Framework
# knowledge, served at /AGENTS.md as a public file.
COPY  docs/AGENTS.md    /app/public/AGENTS.md

ENV NODE_ENV=production

RUN npm install
RUN npm run build

# Stage 2: Serve with Static Server
FROM zopdev/static-server:v0.0.9

# Copy static files from the builder stage
COPY --from=builder /app/out static

# Expose the port server is running on
EXPOSE 8000

# Start go server
CMD ["/main"]