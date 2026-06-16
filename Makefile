# VaniSync-Beckn Gateway Library — build helpers

TLA_MODULE := specs/VaniSyncOutbox.tla
TLA_CFG    := specs/VaniSyncOutbox.cfg
TLA2TOOLS  ?= tla2tools.jar
DB_PATH    ?= ./data/vanisync.db

.PHONY: test migrate tla-check
test:
	go test ./...

migrate:
	go run ./cmd/migrate -db "$(DB_PATH)"

tla-check:
	@test -f "$(TLA2TOOLS)" || (echo "Set TLA2TOOLS to path of tla2tools.jar (download from https://github.com/tlaplus/tlaplus/releases)" && exit 1)
	java -cp "$(TLA2TOOLS)" tlc2.TLC -config "$(TLA_CFG)" "$(TLA_MODULE)"
