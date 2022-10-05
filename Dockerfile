FROM golang:1.18

WORKDIR /go/src/app
COPY .  .

RUN go get -d -v ./...
RUN go install -v ./...

RUN apt-get update
RUN apt-get install -y ffmpeg

EXPOSE 8083

ENV GO111MODULE=on
ENV GIN_MODE=release

CMD go run *.go