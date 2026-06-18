module github.com/tabnas/railroad/go

go 1.24.7

require (
	github.com/tabnas/json/go v0.0.0
	github.com/tabnas/parser/go v0.0.0
)

// This package introspects the tabnas engine and the @tabnas/json grammar.
// Until tabnas/parser and tabnas/json publish tagged Go modules, depend on
// sibling checkouts — the same development model the TypeScript package
// uses (file:../../parser/ts). Clone https://github.com/tabnas/parser and
// https://github.com/tabnas/json as siblings of this repo.
replace github.com/tabnas/parser/go => ../../parser/go

replace github.com/tabnas/json/go => ../../json/go
