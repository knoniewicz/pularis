FROM golang:1.25.3-alpine AS build

WORKDIR /src

COPY core/go.mod core/go.sum ./
RUN go mod download

COPY core/ ./
RUN go build -o /out/pularis .

FROM alpine:3.22

WORKDIR /app

RUN adduser -D -H -u 10001 appuser

COPY --from=build /out/pularis /app/pularis

USER appuser

ENV CONFIGS_DIR=/app/configs

ENTRYPOINT ["/app/pularis"]
