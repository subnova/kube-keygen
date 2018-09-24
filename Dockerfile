FROM golang:alpine as build

RUN apk -v --update add ca-certificates

ADD . $GOPATH/src/github.com/subnova/kube-keygen
RUN cd $GOPATH/src/github.com/subnova/kube-keygen && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/src/github.com/subnova/kube-keygen/kube-keygen /kube-keygen

ENTRYPOINT ["/kube-keygen"]
CMD []
