module github.com/arturoeanton/go-notebook

go 1.25.5

require (
	github.com/arturoeanton/go-dsl v1.4.0
	github.com/arturoeanton/go-echo-live-view v0.2.0
	github.com/arturoeanton/go-pandas v1.0.0
	github.com/arturoeanton/go-xml v1.2.0
	github.com/cosmos72/gomacro v0.0.0-20251011163654-53dd3d49f67b
	github.com/gomarkdown/markdown v0.0.0-20260614204949-e08cff860f76
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.0
	github.com/labstack/echo/v4 v4.13.4
	github.com/lib/pq v1.12.3
	golang.org/x/crypto v0.38.0
	modernc.org/sqlite v1.53.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/labstack/gommon v0.5.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/peterh/liner v1.2.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/tools v0.45.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

// x/tools must stay at the version gomacro requires: newer gcimporters always
// materialize types.Alias, which gomacro's import scanner cannot handle yet
// ("unexpected type ..." when importing packages at runtime). x/text is kept
// at the version gomacro's runtime plugins resolve, so plugin.Open accepts
// them (host and plugin must share identical package versions).
replace golang.org/x/tools => golang.org/x/tools v0.25.1
