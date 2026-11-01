# syntax=docker/dockerfile:1
FROM node:22-alpine AS frontend
WORKDIR /src
COPY package.json package-lock.json .npmrc ./
RUN npm ci --no-audit --no-fund
COPY webpack.config.ts tailwind.config.ts tsconfig.json ./
COPY web_src web_src
COPY templates templates
RUN npx webpack

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/public/assets public/assets
RUN CGO_ENABLED=0 go build -trimpath -tags bindata -ldflags '-s -w' -o /out/dbaccess .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /out/dbaccess /app/dbaccess
ENV DBACCESS_HTTP_ADDR=:8080 DBACCESS_DB_TYPE=postgres
EXPOSE 8080
ENTRYPOINT ["/app/dbaccess", "web"]
