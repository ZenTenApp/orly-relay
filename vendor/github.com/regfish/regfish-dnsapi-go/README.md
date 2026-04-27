# regfish DNS API Go Client

[![Go Reference](https://pkg.go.dev/badge/test.svg)](https://pkg.go.dev/github.com/regfish/regfish-dnsapi-go)

# Testing

Create a `.env` file containing the varibles `RF_API_KEY` using credentials from your regfish account (from Account, Security, API keys). Modify `client_test.go` and replace `example.com` with your own domain, then run `go test -v` to run the tests.