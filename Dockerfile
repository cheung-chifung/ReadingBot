FROM alpine:3.5
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /app
ADD dist/bot_amd64 /app

CMD exec /app/bot_amd64