FROM golang:latest

LABEL maintainer="logdog@teejay.me"

RUN mkdir -p /app
WORKDIR /app
COPY . .
RUN go mod download

CMD ["make", "run-local"]