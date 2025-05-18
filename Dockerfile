FROM golang:1.24-alpine as builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go build -o ./invest_helper_bot ./cmd/main.go

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache mailcap

COPY --from=builder /build/invest_helper_bot /app/
COPY --from=builder /build/.env /app/
COPY --from=builder /build/migrations /app/migrations
COPY --from=builder /build/googleCredentials.json /app/

CMD ["./invest_helper_bot"]