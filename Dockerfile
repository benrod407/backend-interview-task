
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/main.go


FROM alpine:latest
RUN apk --no-cache add ca-certificates
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser
WORKDIR /app
COPY --from=builder /app/server .
RUN chown -R appuser:appuser /app
USER appuser


EXPOSE 9001
CMD ["./server"]
