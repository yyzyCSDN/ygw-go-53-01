FROM golang:1.23

WORKDIR /app

ENV GOPROXY=off \
    GOSUMDB=off

COPY . .
RUN go build -mod=vendor ./...

CMD ["bash"]
