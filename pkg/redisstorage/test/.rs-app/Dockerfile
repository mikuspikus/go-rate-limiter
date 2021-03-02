FROM golang:1.14.4-alpine

RUN mkdir /redis-storage
WORKDIR /redis-storage
COPY . .
CMD CGO_ENABLED=0 go test ./...