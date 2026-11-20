# Release image built by GoReleaser from its own binaries (.goreleaser.yaml);
# the Dockerfile at the root builds the same image from source.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/horizon /horizon
EXPOSE 8080 9090
USER nonroot:nonroot
ENTRYPOINT ["/horizon"]
CMD ["serve"]
