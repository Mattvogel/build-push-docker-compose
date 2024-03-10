FROM golang:alpine as builder

RUN apk update && apk add --no-cache git
WORKDIR /go/src/build-push-docker-compose
COPY . .
RUN go get -d -v
RUN go build -o /go/bin/build-push-docker-compose .

FROM scratch
COPY --from=builder /go/bin/build-push-docker-compose /opt/build-push-docker-compose
WORKDIR /github/workspace
ENTRYPOINT ["/opt/build-push-docker-compose"]
