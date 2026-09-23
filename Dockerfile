FROM node:24-alpine AS web-build
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN mkdir -p /state

# distroless/static includes CA certificates for HTTPS calls to the LLM provider.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
COPY --chown=nonroot:nonroot --from=build /state /state
COPY --from=web-build /src/web/dist /web
ENV ADDR=:8080
ENV WEB_DIR=/web
ENV AUTOPILOT_BUDGET_FILE=/state/usage.json
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
