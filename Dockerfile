FROM golang:1.22.0-alpine as builder

WORKDIR /build
COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go mod download
RUN go build -o ./invest_helper_bot ./cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /build/invest_helper_bot /app/
COPY --from=builder /build/.env /app/

CMD ["./invest_helper_bot"]