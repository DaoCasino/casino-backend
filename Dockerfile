FROM golang:latest AS builder
ENV GO111MODULE=on
RUN mkdir -p /build
ADD . /build
WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o casino .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /build/casino /usr/bin
CMD ["casino"]