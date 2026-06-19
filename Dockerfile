# ---- build stage ----
FROM golang:1.22-alpine AS build
WORKDIR /src

# Copy module definition first for better layer caching.
COPY go.mod ./
RUN go mod download

COPY . .
# Static build so the final image needs no libc.
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

# ---- run stage ----
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/server /server
EXPOSE 8080
ENV PORT=8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
