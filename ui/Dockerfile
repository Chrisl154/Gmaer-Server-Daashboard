# ── Build stage ─────────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY package.json package-lock.json ./
RUN npm ci

# Build
COPY . .
ARG VITE_DAEMON_URL=https://localhost:8443
ENV VITE_DAEMON_URL=${VITE_DAEMON_URL}
RUN npm run build

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM nginx:1.25-alpine

LABEL org.opencontainers.image.title="Games Dashboard UI"
LABEL org.opencontainers.image.description="Web UI for the Gaming Server Dashboard"
LABEL org.opencontainers.image.source="https://github.com/games-dashboard/games-dashboard"
LABEL org.opencontainers.image.licenses="MIT"

# Remove default nginx config
RUN rm /etc/nginx/conf.d/default.conf

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# nginx config: SPA with API proxy to daemon
COPY <<'EOF' /etc/nginx/conf.d/games-dashboard.conf
server {
    listen 80;
    listen [::]:80;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    # Serve static files; fallback to index.html for SPA routing
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Health check endpoint
    location /healthz {
        access_log off;
        return 200 '{"status":"ok"}';
        add_header Content-Type application/json;
    }

    # Gzip static assets
    gzip on;
    gzip_types text/html text/css application/javascript application/json image/svg+xml;
    gzip_min_length 1024;

    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header Referrer-Policy strict-origin-when-cross-origin;
    add_header Content-Security-Policy "default-src 'self'; connect-src 'self' https: wss:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'";
}
EOF

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
