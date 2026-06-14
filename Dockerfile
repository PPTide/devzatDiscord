# Fetch
FROM golang:1.25 AS fetch-stage
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Build
FROM fetch-stage AS build-stage
WORKDIR /app
COPY *.go ./
RUN --mount=type=cache,target="/root/.cache/go-build" GOCACHE=/root/.cache/go-build CGO_ENABLED=1 GOOS=linux go build -o /app/app

# Deploy
FROM gcr.io/distroless/base-debian12 AS deploy-stage
COPY --from=build-stage /app/app /app
EXPOSE 8095
WORKDIR /data
ENTRYPOINT ["/app"]