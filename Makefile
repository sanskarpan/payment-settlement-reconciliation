GO ?= go

.PHONY: test build race vet profile before after end-to-end dump-restore submission-smoke

build:
	$(GO) build -o bin/recon ./cmd/recon

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

profile:
	$(GO) run ./cmd/recon profile --payments workingData/amazon_payments_data.csv --settlements workingData/amazon_settlements_data.txt

before: build
	./bin/recon run --payments workingData/amazon_payments_data.csv --settlements workingData/amazon_settlements_data.txt --payment-config workingData/amazon_payment_configs_au_old.csv --settlement-config workingData/amazon_settlement_configs_au.csv --settlement-id 12395580393 --output output/before_fix.xlsx --mode diagnostic-baseline

after: build
	mkdir -p output/fixed
	./bin/recon patch-config --input workingData/amazon_payment_configs_au_old.csv --source payment --output output/fixed/payment_configs.csv
	./bin/recon patch-config --input workingData/amazon_settlement_configs_au.csv --source settlement --output output/fixed/settlement_configs.csv
	./bin/recon run --payments workingData/amazon_payments_data.csv --settlements workingData/amazon_settlements_data.txt --payment-config output/fixed/payment_configs.csv --settlement-config output/fixed/settlement_configs.csv --settlement-id 12395580393 --output output/after_fix.xlsx --mode strict
	./bin/recon verify --report output/after_fix.xlsx

end-to-end:
	./tools/end_to_end.sh

dump-restore:
	./tools/dump_restore_smoke.sh

submission-smoke:
	./tools/submission_smoke.sh
