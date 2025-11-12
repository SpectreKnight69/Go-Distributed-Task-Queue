# -------- Build stage --------
FROM golang:1.25.3-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/main.go

# -------- Runtime stage --------
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/server .
EXPOSE 8080
ENV REDIS_ADDR=redis:6379
CMD ["/app/server"]
