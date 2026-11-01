# SPDX-License-Identifier: MIT

FROM golang:1.27.3-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN mkdir -p /out \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
      -o /out/uptimefoundry ./cmd/uptimefoundry \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
      -o /out/uptimefoundry-agent ./cmd/uptimefoundry-agent

FROM gcr.io/distroless/static-debian12:nonroot AS control-plane
COPY --from=build /out/uptimefoundry /usr/local/bin/uptimefoundry
COPY configs/uptimefoundry.example.yaml /etc/uptimefoundry/config.yaml
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/uptimefoundry", "-config", "/etc/uptimefoundry/config.yaml"]

FROM gcr.io/distroless/static-debian12:nonroot AS agent
COPY --from=build /out/uptimefoundry-agent /usr/local/bin/uptimefoundry-agent
ENTRYPOINT ["/usr/local/bin/uptimefoundry-agent"]
