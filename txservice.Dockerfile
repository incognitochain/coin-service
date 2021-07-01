FROM golang:1.16.2-stretch AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN cd txservice && sh build.sh prod


FROM alpine

WORKDIR /app

COPY --from=build /app/txservice/txservice /app/txservice

CMD [ "./txservice" ]
