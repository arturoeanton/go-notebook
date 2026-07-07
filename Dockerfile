FROM golang:1.26-alpine
RUN apk add --no-cache build-base
WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build .
RUN go get gonum.org/v1/plot
RUN go get gonum.org/v1/plot/vg
RUN go get gonum.org/v1/plot/plotter
RUN go get gonum.org/v1/plot/plotutil

# Cells run arbitrary Go code with the server's privileges: keep it non-root.
RUN adduser -D gonote && chown -R gonote /app /go
USER gonote

CMD ["/app/go-notebook", "-p", "0.0.0.0:1323"]

## docker login docker.io
## docker build .  -t arturoeanton/go-notebook
## docker push arturoeanton/go-notebook
## docker run --rm -p 1323:1323 go-notebook
