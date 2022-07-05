FROM golang:1.18-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

RUN go build -o /epa

FROM alpine:3.16

WORKDIR /

COPY --from=build /epa /epa

VOLUME /storage

CMD [ "/epa" ]
