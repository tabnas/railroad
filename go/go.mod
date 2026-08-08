module github.com/tabnas/railroad/go

go 1.24.7

require (
	github.com/tabnas/json/go v0.4.3

	// NOTE: extract.go calls (*Tabnas).RuleNames, added AFTER v0.6.1, so this
	// requirement is behind the source. Local builds and CI both resolve the
	// sibling engine through a go.work, but `GOWORK=off go build ./...` fails
	// until this line is bumped to the release carrying RuleNames. See
	// ../AGENTS.md, "Rule order".
	github.com/tabnas/parser/go v0.6.2
)
