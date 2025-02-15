# Build stage
FROM golang:1.24 as builder
WORKDIR /app
COPY . .
RUN make build

# Final stage
FROM alpine:latest
COPY --from=builder /app/bin/sla-monitor /usr/local/bin/sla-monitor
ENTRYPOINT ["sla-monitor"]