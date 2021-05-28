FROM golang:1.16-alpine as builder
RUN apk add --no-cache make gcc git musl-dev

COPY . /src
RUN make -C /src install PREFIX=/pkg GO_BUILDFLAGS='-mod vendor'

################################################################################

FROM alpine:latest
LABEL source_repository="https://github.com/sapcc/absent-metrics-operator"

RUN apk add --no-cache ca-certificates
COPY --from=builder /pkg/ /usr/
ENTRYPOINT [ "/usr/bin/absent-metrics-operator" ]
