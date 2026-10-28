GO                 = go
GOFMT              = gofmt
CURL               = curl

FUZZ_TIME         ?= 30s
FUZZ_TARGETS       = ./entsoe:FuzzDecode

# ENTSO-E's API, and France's bidding zone, for make testdata.
ENTSOE_API         = https://web-api.tp.entsoe.eu/api
FR                 = 10YFR-RTE------C
PRICES             = documentType=A44&in_Domain=$(FR)&out_Domain=$(FR)&contract_MarketAgreement.type=A01
LOAD               = documentType=A65&processType=A01&outBiddingZone_Domain=$(FR)
WIND_SOLAR         = documentType=A69&processType=A01&in_Domain=$(FR)
# Local days in Europe/Paris, as UTC: 27 and 28 Oct 2026, the 25 h day clocks went back, the 23 h one they went
# forward, and a Sunday of negative prices.
OCT                = periodStart=202610262300&periodEnd=202610282300
DST_END            = periodStart=202610242200&periodEnd=202610252300
DST_START          = periodStart=202603282300&periodEnd=202603292200
MAY                = periodStart=202605092200&periodEnd=202605102200
FAR                = periodStart=209901012300&periodEnd=209901022300

# For CI
ifneq ($(wildcard ./bin/golangci-lint),)
	GOLINT = $(CURDIR)/bin/golangci-lint
else
	GOLINT = golangci-lint
endif

V                 = 0
Q                 = $(if $(filter 1,$V),,@)
M                 = $(shell printf "\033[0;35m▶\033[0m")

.PHONY: all
all: check

# Tidy
.PHONY: tidy
tidy: ## Run go mod tidy
	$(info $(M) running go mod tidy) @
	$Q $(GO) mod tidy

# Check
.PHONY: check
check: lint test ## Run lint and tests

# Lint
.PHONY: lint
lint: ## Run golangci-lint
	$(info $(M) running $(GOLINT))
	$Q $(GOLINT) run

# Test
.PHONY: test
# -cpu 1,4 varies interleavings, -shuffle catches tests that depend on each other.
test: ## Run tests with race detector, shuffled, with GOMAXPROCS 1 and 4
	$(info $(M) running go test) @
	$Q $(GO) test -cover -race -shuffle=on -cpu 1,4 -v ./...

# Fuzz
.PHONY: fuzz
# The seeds are ENTSO-E's documents, up to 60 KB, whose mutants the minimizer would spend most of FUZZ_TIME on.
fuzz: ## Run each fuzz target for FUZZ_TIME (30s)
	$(info $(M) running each fuzz target for $(FUZZ_TIME)) @
	$Q for t in $(FUZZ_TARGETS); do \
		echo "$(M) $$t"; \
		$(GO) test -run '^$$' -fuzz "^$${t##*:}\$$" -fuzztime $(FUZZ_TIME) -fuzzminimizetime 5s $${t%%:*} || exit 1; \
	done

# Testdata
.PHONY: testdata
# The token comes from the environment and is never echoed: the documents ENTSO-E answers do not hold it.
# testdata/a44_overlap.xml is written by hand, and examples/01-day-ahead keeps its own copy of the May day.
testdata: ## Record ENTSO-E's answers into testdata/, with ENTSOE_TOKEN
	$(info $(M) recording ENTSO-E's answers into testdata) @
	$Q test -n "$$ENTSOE_TOKEN" || { echo "ENTSOE_TOKEN is not set"; exit 1; }
	$Q set -e; \
	get() { $(CURL) -sS -o "testdata/$$1" "$(ENTSOE_API)?securityToken=$$2&$$3"; }; \
	get a44_fr_20261027.xml "$$ENTSOE_TOKEN" '$(PRICES)&$(OCT)'; \
	get a44_fr_20261025.xml "$$ENTSOE_TOKEN" '$(PRICES)&$(DST_END)'; \
	get a44_fr_20260329.xml "$$ENTSOE_TOKEN" '$(PRICES)&$(DST_START)'; \
	get a44_fr_20260510.xml "$$ENTSOE_TOKEN" '$(PRICES)&$(MAY)'; \
	get a65_fr_20261027.xml "$$ENTSOE_TOKEN" '$(LOAD)&$(OCT)'; \
	get a69_fr_20261027.xml "$$ENTSOE_TOKEN" '$(WIND_SOLAR)&$(OCT)'; \
	get ack_nodata.xml "$$ENTSOE_TOKEN" '$(PRICES)&$(FAR)'; \
	get ack_auth.xml 00000000-0000-0000-0000-000000000000 '$(PRICES)&$(OCT)'; \
	cp testdata/a44_fr_20260510.xml examples/01-day-ahead/testdata/

## Helpers

.PHONY: go-version
go-version: ## Print go version used in this makefile
	$Q echo $(GO)

.PHONY: fmt
fmt: ## Format code
	$(info $(M) running $(GOFMT)) @
	$Q $(GOFMT) -l -w .

.PHONY: doc
doc: ## Print package documentation
	$(info $(M) running go doc) @
	$Q $(GO) doc -all . && $(GO) doc -all ./entsoe

.PHONY: help
help: ## Print this
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
