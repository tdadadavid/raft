FROM golang:latest
WORKDIR /app
COPY . .
RUN go mod tidy
EXPOSE 8001 8002 8003
CMD ["go", "run", "main.go"]