# Release image: one static binary, non-root (docs/operations.md).
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/lidisoft/horizon/internal/version.Version=${VERSION} -X github.com/lidisoft/horizon/internal/version.Commit=${COMMIT}" \
      -o /out/horizon ./cmd/horizon

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/horizon /horizon
EXPOSE 8080 9090
USER nonroot:nonroot
ENTRYPOINT ["/horizon"]
CMD ["serve"]
