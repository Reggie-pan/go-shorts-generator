FROM golang:1.24-bullseye AS backend-builder
RUN apt-get update && apt-get install -y ffmpeg espeak curl fonts-noto-cjk fontconfig && rm -rf /var/lib/apt/lists/*
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
ENV TZ=Asia/Taipei
RUN apt-get update && apt-get install -y ffmpeg espeak curl ca-certificates fonts-noto-cjk fontconfig tzdata && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone && \
    rm -rf /var/lib/apt/lists/*
ENV PORT=8080
ENV STORAGE_PATH=/data
ENV BGM_PATH=/assets/bgm
WORKDIR /app
COPY --from=backend-builder /app/server /app/server
COPY --from=frontend-builder /app/frontend/dist /app/public
COPY frontend/swagger.html /app/public/swagger.html
COPY docs /app/docs
COPY assets /assets
VOLUME ["/data"]
EXPOSE 8080
CMD ["/app/server"]
