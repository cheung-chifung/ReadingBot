BIN := dist/bot
BIN_AMD64 := dist/bot_amd64

$(BIN):
	@go build -o $@ .
	@chmod +x $@

$(BIN_AMD64):
	@go build -o $@ .
	@chmod +x $@

docker: clean $(BIN_AMD64)
	@docker build -t docker.io/$(DOCKER_IMAGE_ID) .

docker-push:
	@docker push docker.io/$(DOCKER_IMAGE_ID)

clean:
	@rm -f $(BIN)
	@rm -f $(BIN_AMD64)
