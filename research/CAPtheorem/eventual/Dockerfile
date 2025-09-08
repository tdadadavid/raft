FROM golang:latest
WORKDIR /app
COPY . .
RUN go mod tidy
EXPOSE  9001 9002 9003
CMD ["go", "run", "main.go"]