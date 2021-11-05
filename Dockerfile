FROM golang:1.17.2-stretch AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static" -o coinservice


FROM alpine

WORKDIR /app

COPY --from=build /app/coinservice /app/coinservice

COPY ./devenv/csv/run.sh /app/run.sh

# ADD ./devenv/csv/config /app/config

CMD [ "./coinservice" ]
