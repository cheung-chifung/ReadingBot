linux_amd64 := GOOS=linux GOARCH=amd64
gobuild := go build

BIN := dist/bot
BIN_AMD64 := dist/bot_amd64

$(BIN):
	@$(gobuild) -o $@ .
	@chmod +x $@

$(BIN_AMD64):
	@$(linux_amd64) $(gobuild) -o $@ .
	@chmod +x $@

docker: clean $(BIN_AMD64)
	@docker build -t docker.io/$(DOCKER_IMAGE_ID) .

docker-push:
	@docker push docker.io/$(DOCKER_IMAGE_ID)

clean:
	@rm -f $(BIN)
