FROM golang:1.20.1 as base

FROM base as dev

# Install Air (for development)
RUN curl -sSfL https://raw.githubusercontent.com/cosmtrek/air/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

WORKDIR /opt/app/api

RUN groupadd -g 1002 -o estoraje
RUN useradd -m -u 1002 -g 1002 -o -s /bin/bash estoraje
RUN chown -R 1002:1002 /opt/app/api

USER estoraje
CMD ["air"]
