# vegadns

This is a fork of [vegadns2client](https://github.com/opendns/vegadns2client).

vegadns is a go client for [VegaDNS-API](https://github.com/shupp/VegaDNS-API).

This is an incomplete client, initially intended to support [lego](https://github.com/xenolf/lego).

## Example Usage

### Getting a domain ID

```go
package main

import (
	"context"
	"fmt"

	"github.com/nrdcg/vegadns"
)

func main() {
	client, err := vegadns.NewClient("http://localhost:5000", vegadns.WithOAuth("mykey", "mysecret"))
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	
	domainID, err := client.GetDomainID(ctx, "example.com")
	if err != nil {
		panic(err)
	}

	fmt.Println("domainID:", domainID)
}
```

Which will output the following:

```
domainID: 1
```

### Creating and deleting a TXT record

```go
package main

import (
	"context"
	"fmt"

	"github.com/nrdcg/vegadns"
)

func main() {
	client, err := vegadns.NewClient("http://localhost:5000", vegadns.WithOAuth("mykey", "mysecret"))
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	domainID, err := client.GetDomainID(ctx, "example.com")
	if err != nil {
		panic(err)
	}

	fmt.Println("domainID:", domainID)

	err = client.CreateTXTRecord(ctx, domainID, "_acme-challenge.example.com", "test challenge", 25)
	if err != nil {
		panic(err)
	}

	recordID, err := client.GetRecordID(ctx, domainID, "_acme-challenge.example.com", "TXT")
	if err != nil {
		panic(err)
	}

	fmt.Println("recordID:", recordID)

	err = client.DeleteRecord(ctx, recordID)
	if err != nil {
		panic(err)
	}
}
```

Which will output the following:

```
domainID: 1
recordID: 3
```
