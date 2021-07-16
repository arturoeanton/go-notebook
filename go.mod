module github.com/arturoeanton/go-notebook

go 1.13

require (
	github.com/arturoeanton/go-echo-live-view v0.0.0-20210716154008-04afddacb712
	github.com/arturoeanton/gocommons v0.0.0-20210613045617-59f224587f91
	github.com/cosmos72/gomacro v0.0.0-20210624153544-b4935e406a41
	github.com/gomarkdown/markdown v0.0.0-20210514010506-3b9f47219fe7
	github.com/google/uuid v1.3.0
	github.com/labstack/echo/v4 v4.4.0
	golang.org/x/text v0.3.5 // indirect
)

replace github.com/mattn/go-runewidth => github.com/mattn/go-runewidth v0.0.12

replace github.com/peterh/liner => github.com/peterh/liner v1.2.1

replace golang.org/x/tools => golang.org/x/tools v0.1.0

exclude golang.org/x/text v0.3.6
