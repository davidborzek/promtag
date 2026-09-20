FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /promtag ./cmd/promtag

FROM gcr.io/distroless/static:nonroot
COPY --from=build /promtag /promtag
USER nonroot:nonroot
ENTRYPOINT ["/promtag"]
