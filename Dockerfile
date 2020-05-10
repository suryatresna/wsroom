FROM golang:1.14

RUN apt-get update --fix-missing \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    net-tools \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /go/src/github.com/suryatresna/multiplayerengine

COPY ./ /go/src/github.com/suryatresna/multiplayerengine

RUN go mod download

RUN go get github.com/githubnemo/CompileDaemon

ENTRYPOINT CompileDaemon --build="go build -o bin/main cmd/ws/main.go" --command=./bin/main