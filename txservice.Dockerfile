FROM golang:1.16.2-stretch AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN cd txservice \
    && go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static" -o txservice

FROM alpine

WORKDIR /app

COPY --from=build /app/txservice/txservice /app/txservice

CMD [ "./txservice" ]
