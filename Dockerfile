FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN go build -o /out/wt-media-cloud ./cmd/server

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/wt-media-cloud /usr/local/bin/wt-media-cloud
ENTRYPOINT ["/usr/local/bin/wt-media-cloud"]
