FROM alpine:3

RUN apk fix && \
    apk --no-cache --update add curl

WORKDIR /lb

RUN wget `curl -s https://api.github.com/repos/yyyar/gobetween/releases | grep browser_download_url | grep linux_amd64 | cut -d '"' -f 4 | head -n 1`

RUN tar -zxvf *.tar.gz

COPY ./docker/load-balancer/gobetween.toml config/gobetween.toml

CMD ./gobetween -c config/gobetween.toml
