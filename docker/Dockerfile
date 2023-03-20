FROM golang:1.20.1 as base

FROM base as dev

# Install Air (for development)
RUN curl -sSfL https://raw.githubusercontent.com/cosmtrek/air/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

WORKDIR /opt/app/api

CMD ["air"]
