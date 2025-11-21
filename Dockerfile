FROM golang:1.21-bullseye AS backend-builder
RUN apt-get update && apt-get install -y ffmpeg espeak curl && rm -rf /var/lib/apt/lists/*
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/. .
RUN go build -o /app/server ./cmd/server

FROM node:18-bullseye AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/. .
RUN npm run build

FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y ffmpeg espeak curl ca-certificates && rm -rf /var/lib/apt/lists/*
ENV PORT=8080
ENV STORAGE_PATH=/data
ENV BGM_PATH=/assets/bgm
WORKDIR /app
COPY --from=backend-builder /app/server /app/server
COPY --from=frontend-builder /app/frontend/dist /app/public
COPY docs /app/docs
COPY assets /assets
VOLUME ["/data"]
EXPOSE 8080
CMD ["/app/server"]
