FROM golang:1.18-alpine3.16 AS build

RUN apk update && apk add gcc musl-dev gcompat libc-dev linux-headers

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static" -o coinservice


FROM alpine:3.16

WORKDIR /app

COPY --from=build /app/coinservice /app/coinservice

COPY ./devenv/csv/run.sh /app/run.sh

# ADD ./devenv/csv/config /app/config

CMD [ "./coinservice" ]
