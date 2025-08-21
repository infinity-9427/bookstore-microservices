# --- build stage ---
FROM golang:1.25-alpine AS build
WORKDIR /src

# deps first (cache-friendly)
COPY go.mod go.sum ./
RUN go mod download

# now copy code and make sure new imports are recorded
COPY . .
RUN go mod tidy

# build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app ./cmd/api

# --- runtime stage ---
FROM gcr.io/distroless/base-debian12
COPY --from=build /app /app
USER 1000:1000
EXPOSE 8082
ENTRYPOINT ["/app"]
