FROM golang:alpine3.23 as builder

# deinitializing GOPATH as otherwise go modules don't work properly
ENV GOPATH=""

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /configmapper -trimpath -ldflags="-s -w -extldflags '-static'"

FROM alpine:3.23

COPY --from=builder /configmapper /
ENTRYPOINT [ "/configmapper" ]
