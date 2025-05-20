FROM golang:1.21-alpine as builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy && go mod download

COPY . .
RUN go build -v -o imsi-switch-receiver .

FROM alpine
WORKDIR /root/
COPY --from=builder /app/imsi-switch-receiver .
CMD ["./imsi-switch-receiver"] 