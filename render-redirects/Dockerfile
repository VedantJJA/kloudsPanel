FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/redirector ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/redirector /redirector
ENTRYPOINT ["/redirector"]
