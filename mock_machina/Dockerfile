FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-s -w -X github.com/demola234/tiny-tools/mock_machina/internal/buildinfo.version=${VERSION}" \
    -o /out/mockmachina ./cmd/mockmachina

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.title="MockMachina" \
      org.opencontainers.image.description="Mock HTTP APIs from contract files in your repository" \
      org.opencontainers.image.source="https://github.com/demola234/tiny-tools" \
      org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build /out/mockmachina /usr/local/bin/mockmachina
WORKDIR /mock
EXPOSE 4001
ENTRYPOINT ["mockmachina"]
CMD ["start", "--plain", "--host", "0.0.0.0", "--dir", "/mock/.mockmachina", "--watch-interval", "500ms"]
