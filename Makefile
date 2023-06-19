ENV=CGO_ENABLED=0
CC=go build
CFLAGS=-ldflags "-s -w"
APPNAME=demo
TARGET=bin/$(APPNAME)
SRC=cmd/$(APPNAME)/main.go


all:  build-all dockerize compose

docker:
	docker build -t smdb .

dockerize:
	cp bin/edgy docker/edgy
	cp bin/cloudy docker/cloudy
	cp bin/demo docker/demo

compose:
	docker compose down
	docker compose build
	docker compose up 

build-all:  build-edgy build-cloudy build-demo

APPNAME=edgy
build-edgy: 
	$(ENV) $(CC) $(CFLAGS) -o bin/edgy cmd/edgy/main.go

build-cloudy: 
	$(ENV) $(CC) $(CFLAGS) -o bin/cloudy cmd/cloudy/main.go 

build-demo:
	$(ENV) $(CC) $(CFLAGS) -o bin/demo cmd/demo/main.go 
