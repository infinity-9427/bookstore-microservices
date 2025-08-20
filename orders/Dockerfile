# orders/Dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src

# cache deps
COPY go.mod go.sum ./
RUN go mod download

# now copy code and build
COPY . .
RUN CGO_ENABLED=0 go build -o /app ./cmd/api

FROM gcr.io/distroless/base-debian12
ENV PORT=8082
EXPOSE 8082
COPY --from=build /app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
