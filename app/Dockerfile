# Builds the web client as a Wails server-mode binary: the same frontend and
# Go services as the native app, served over HTTP to ordinary browsers.
FROM node:24-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY --from=frontend /app/frontend/dist ./frontend/dist
ENV CGO_ENABLED=0
RUN go build -tags server,production -trimpath -ldflags="-s -w" -o /prayer-deck-web .

FROM gcr.io/distroless/static AS app
COPY --from=build /prayer-deck-web /prayer-deck-web
ENV WAILS_SERVER_HOST=0.0.0.0
EXPOSE 8080
ENTRYPOINT ["/prayer-deck-web"]
