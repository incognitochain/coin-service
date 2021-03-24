FROM golang:1.16.2-stretch AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -ldflags "-linkmode external -extldflags -static" -o coinservice


FROM alpine

COPY --from=build /app/coinservice /app/coinservice

CMD [ "./coinservice" ]
