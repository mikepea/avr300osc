.DEFAULT_GOAL := help

## -- Section Delimiter --

## This help message
## Which can also be multiline
.PHONY: help
help:
	@printf "Usage\n";

	@awk '{ \
			if ($$0 ~ /^.PHONY: [a-zA-Z\-\_0-9]+$$/) { \
				helpCommand = substr($$0, index($$0, ":") + 2); \
				if (helpMessage) { \
					printf "\033[36m%-20s\033[0m %s\n", \
						helpCommand, helpMessage; \
					helpMessage = ""; \
				} \
			} else if ($$0 ~ /^[a-zA-Z\-\_0-9.]+:/) { \
				helpCommand = substr($$0, 0, index($$0, ":")); \
				if (helpMessage) { \
					printf "\033[36m%-20s\033[0m %s\n", \
						helpCommand, helpMessage; \
					helpMessage = ""; \
				} \
			} else if ($$0 ~ /^##/) { \
				if (helpMessage) { \
					helpMessage = helpMessage"\n                     "substr($$0, 3); \
				} else { \
					helpMessage = substr($$0, 3); \
				} \
			} else { \
				if (helpMessage) { \
					print "\n                     "helpMessage"\n" \
				} \
				helpMessage = ""; \
			} \
		}' \
		$(MAKEFILE_LIST)

## -- QA Task Runners --

## Run linter
.PHONY: lint
lint:
	exit 0


## Run unit and integration tests
.PHONY: test
test:
	exit 0


## Build the server binary
.PHONY: build
build:
	go build

## Build specifically for Raspberry Pi
.PHONY: build/pi
build/pi:
	env GOOS=linux GOARCH=arm GOARM=5 go build

## install to /usr/local/bin
.PHONY: install
install: build
	sudo install -m 0755 avr300osc /usr/local/bin/avr300osc

## deploy to systemd
.PHONY: deploy-systemd
deploy-systemd: install
	sudo install -m 0644 systemd/avr300osc.service /etc/systemd/system/
	sudo systemctl enable avr300osc
	sudo systemctl daemon-reload

## deploy
.PHONY: deploy
deploy: install
	sudo systemctl restart avr300osc

.PHONY: remote-deploy
remote-deploy: build/pi
	scp avr300osc mikepea@audio-pi:/tmp/
	ssh mikepea@audio-pi sudo install -o 0755 /tmp/avr300osc /usr/local/bin/
	ssh mikepea@audio-pi sudo systemctl restart avr300osc

## tail systemd logs
.PHONY: status
status:
	systemctl status avr300osc

## tail systemd logs
.PHONY: logtail
logtail:
	sudo journalctl -f -u avr300osc

## Tidy up build files
.PHONY: clean
clean:
	rm ./avr300osc


