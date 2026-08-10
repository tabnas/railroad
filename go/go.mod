module github.com/tabnas/railroad/go

go 1.24.7

require (
	github.com/tabnas/json/go v0.5.1

	// extract.go calls (*Tabnas).RuleNames; this requirement is at or past the
	// release that added it, so `GOWORK=off go build ./...` works. See
	// ../AGENTS.md, "Rule order".
	github.com/tabnas/parser/go v0.8.1
)
