FROM ghcr.io/gofr-dev/website:latest AS builder

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


# Stage 2: Serve with NGINX
FROM nginx:stable-alpine

# Copy static files from the builder stage
COPY --from=builder /app/out /usr/share/nginx/html
COPY --from=builder /app/nginix.conf /etc/nginx/conf.d/default.conf

# Expose the port NGINX is running on
EXPOSE 3000

# Start NGINX server
CMD ["nginx", "-g", "daemon off;"]