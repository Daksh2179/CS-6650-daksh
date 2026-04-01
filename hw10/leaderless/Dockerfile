FROM golang:1.23 AS builder

WORKDIR /app

COPY node/go.mod ./
RUN go mod download

COPY node/*.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /kv-node

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

COPY --from=builder /kv-node /kv-node

EXPOSE 8080

CMD ["/kv-node"]