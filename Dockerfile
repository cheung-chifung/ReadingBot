FROM alpine:3.5
RUN apk update && apk add ca-certificates sqlite gcc musl-dev && rm -rf /var/cache/apk/*
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

WORKDIR /app
ADD dist/bot_amd64 /app

CMD exec /app/bot_amd64
