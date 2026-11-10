ARG BASE_IMAGE=gcr.io/distroless/static
FROM golang:1.27-alpine AS build

WORKDIR /go/src

# Fetch dependencies first for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
ENV CGO_ENABLED=0
RUN go build -trimpath -o ./prayer-deck-api ./cmd/app

FROM $BASE_IMAGE AS app
COPY --from=build /go/src/prayer-deck-api /prayer-deck-api
ENTRYPOINT ["/prayer-deck-api"]
