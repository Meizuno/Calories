# Client and server are separate projects; they are combined only here, at build.
#
# 1) Build the Vue client (pnpm).
FROM node:22-bookworm AS client
RUN corepack enable
WORKDIR /client
COPY client/package.json client/pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile
COPY client/ ./
RUN pnpm build

# 2) Build the Go server.
FROM golang:1.25-bookworm AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/calories ./cmd/server

# 3) Runtime: the binary + the built client beside it (served at CLIENT_DIR).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/calories /calories
COPY --from=client /client/dist /app/dist
ENV CLIENT_DIR=/app/dist PORT=8080
EXPOSE 8080
ENTRYPOINT ["/calories"]
