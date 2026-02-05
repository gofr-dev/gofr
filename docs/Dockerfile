FROM ghcr.io/gofr-dev/website:latest AS builder

WORKDIR /app

COPY  docs/quick-start /app/src/app/docs/quick-start
COPY  docs/public/ /app/public
COPY  docs/advanced-guide /app/src/app/docs/advanced-guide
COPY docs/datasources /app/src/app/docs/datasources
COPY  docs/references /app/src/app/docs/references
COPY  docs/page.md /app/src/app/docs
COPY  docs/navigation.js /app/src/lib
COPY  docs/events.json /app/src/app/events
COPY  docs/testimonials.json /app/utils

ENV NODE_ENV=production

RUN npm install
RUN npm run build

# Stage 2: Serve with Static Server
FROM zopdev/static-server:v0.0.5

# Copy static files from the builder stage
COPY --from=builder /app/out static

# Expose the port server is running on
EXPOSE 8000

# Start go server
CMD ["/main"]