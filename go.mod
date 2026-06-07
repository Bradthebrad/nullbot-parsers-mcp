module github.com/Bradthebrad/nullbot-parsers-mcp

go 1.26.4

require tinychain/mcp v0.0.0

require (
	tinychain v0.0.0 // indirect
	tinychain/agent v0.0.0 // indirect
)

replace tinychain => ../tinychain/client

replace tinychain/agent => ../tinychain/agent

replace tinychain/mcp => ../tinychain/mcp
