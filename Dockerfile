FROM golang:latest as builder
ADD ./server /go/src/envoy-router/server
WORKDIR /go/src/envoy-router/server
RUN groupadd -r -g 20000 app && useradd -M -u 20001 -g 0 -r -c "Default app user" app && chown -R 20001:0 /go
ENV GO111MODULE=on
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -a -ldflags='-s -w -extldflags "-static"' -o /go/bin/envoy-router /go/src/envoy-router/server/main.go

#without these certificates, we cannot verify the JWT token
FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
WORKDIR /
COPY --from=builder /go/bin/envoy-router .
COPY --from=builder /etc/passwd /etc/group /etc/shadow /etc/
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 20001
EXPOSE 50051
CMD ["./envoy-router"]