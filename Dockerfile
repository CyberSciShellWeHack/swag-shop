FROM golang:1.21-alpine3.18 as builder 
RUN apk update && apk upgrade
RUN apk add build-base
WORKDIR /src
COPY ./src .

# Allows docker to cache after downloads have occurred
RUN go mod download

# ENV GOCACHE=/root/.cache/go-build
# If you can enable DOCKER_BUILDKIT=1
# RUN --mount=type=cache,target="/root/.cache/go-build" go build -o shop main.go items.go orders.go auth.go
RUN GOOS=linux CGO_ENABLED=1 go build -o shop main.go items.go orders.go auth.go

FROM alpine/sqlite:3.41.2
RUN apk update && apk upgrade
RUN apk add build-base
EXPOSE 443
WORKDIR /app
COPY ./files .
RUN chmod 600 admin_public_key.pem
RUN chmod 644 server.crt 
RUN chmod 600 server.key
COPY --from=builder /src/shop /app/shop
ENTRYPOINT [""]
CMD ["/app/shop"]



