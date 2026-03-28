FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app /app
EXPOSE 9119
ENTRYPOINT ["/app"]
