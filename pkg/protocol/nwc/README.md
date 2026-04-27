# NWC Client

Nostr Wallet Connect (NIP-47) client implementation for the ORLY relay. This package provides a complete client for connecting to NWC-compatible lightning wallets, enabling the relay to accept payments through the Nostr protocol.

## Features

- **NIP-47 Compliance**: Full implementation of the Nostr Wallet Connect specification
- **NIP-44 Encryption**: End-to-end encrypted communication with wallets
- **Payment Processing**: Invoice creation, payment, and balance checking
- **Real-time Notifications**: Subscribe to payment events and wallet updates
- **Error Handling**: Comprehensive error handling and recovery
- **Context Support**: Proper Go context integration for timeouts and cancellation
- **Thread Safety**: Concurrent access support with proper synchronization

## Installation

```bash
go get git.smesh.lol/orly/pkg/protocol/nwc
```

## Usage

### Basic Client Setup

```go
import "git.smesh.lol/orly/pkg/protocol/nwc"

// Create client from NWC connection URI
client, err := nwc.NewClient("nostr+walletconnect://...")
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### Making Requests

```go
ctx := context.Background()

// Get wallet information
var info map[string]any
err = client.Request(ctx, "get_info", nil, &info)
if err != nil {
    log.Printf("Failed to get info: %v", err)
}

// Get wallet balance
var balance map[string]any
err = client.Request(ctx, "get_balance", nil, &balance)

// Create invoice
params := map[string]any{
    "amount": 1000,      // msats
    "description": "ORLY Relay Access",
}
var invoice map[string]any
err = client.Request(ctx, "make_invoice", params, &invoice)

// Check invoice status
lookupParams := map[string]any{
    "payment_hash": "abc123...",
}
var status map[string]any
err = client.Request(ctx, "lookup_invoice", lookupParams, &status)

// Pay invoice
payParams := map[string]any{
    "invoice": "lnbc10n1pj...",
}
var payment map[string]any
err = client.Request(ctx, "pay_invoice", payParams, &payment)
```

### Payment Notifications

Subscribe to real-time payment notifications:

```go
// Subscribe to payment notifications
err = client.SubscribeNotifications(ctx, func(notificationType string, notification map[string]any) error {
    switch notificationType {
    case "payment_received":
        amount := notification["amount"].(float64)
        paymentHash := notification["payment_hash"].(string)
        description := notification["description"].(string)
        log.Printf("Payment received: %.2f sats for %s", amount/1000, description)
        // Process payment...

    case "payment_sent":
        amount := notification["amount"].(float64)
        paymentHash := notification["payment_hash"].(string)
        log.Printf("Payment sent: %.2f sats", amount/1000)
        // Handle outgoing payment...

    default:
        log.Printf("Unknown notification type: %s", notificationType)
    }
    return nil
})
```

## API Reference

### Client Methods

#### `NewClient(uri string) (*Client, error)`
Creates a new NWC client from a connection URI.

#### `Request(ctx context.Context, method string, params, result any) error`
Makes a synchronous request to the wallet.

#### `SubscribeNotifications(ctx context.Context, handler NotificationHandler) error`
Subscribes to payment notifications with the provided handler function.

#### `Close() error`
Closes the client and cleans up resources.

### Supported Methods

| Method | Parameters | Description |
|--------|------------|-------------|
| `get_info` | None | Get wallet information and supported methods |
| `get_balance` | None | Get current wallet balance |
| `make_invoice` | `amount`, `description` | Create a new lightning invoice |
| `lookup_invoice` | `payment_hash` | Check status of an existing invoice |
| `pay_invoice` | `invoice` | Pay a lightning invoice |

### Notification Types

- `payment_received`: Incoming payment received
- `payment_sent`: Outgoing payment sent
- `invoice_paid`: Invoice was paid (alternative to payment_received)

## Integration with ORLY

The NWC client is integrated into the ORLY relay for payment processing:

```bash
# Enable NWC payments
export ORLY_NWC_URI="nostr+walletconnect://..."

# Start relay with payment support
./orly
```

The relay will automatically:
- Create invoices for premium features
- Validate payments before granting access
- Handle payment notifications
- Update user balances

## Error Handling

The client provides comprehensive error handling:

```go
err = client.Request(ctx, "make_invoice", params, &result)
if err != nil {
    var nwcErr *nwc.Error
    if errors.As(err, &nwcErr) {
        switch nwcErr.Code {
        case nwc.ErrInsufficientBalance:
            // Handle insufficient funds
        case nwc.ErrInvoiceExpired:
            // Handle expired invoice
        default:
            // Handle other errors
        }
    }
}
```

## Security Considerations

- **URI Protection**: Keep NWC URIs secure and don't log them
- **Context Timeouts**: Always use context with timeouts for requests
- **Error Sanitization**: Don't expose internal wallet errors to users
- **Rate Limiting**: Implement rate limiting to prevent abuse
- **Audit Logging**: Log payment operations for audit purposes

## Testing

The NWC client includes comprehensive tests:

### Running Tests

```bash
# Run NWC package tests
go test ./pkg/protocol/nwc

# Run with verbose output
go test -v ./pkg/protocol/nwc

# Run integration tests (requires wallet connection)
go test -tags=integration ./pkg/protocol/nwc
```

### Integration Testing

Part of the full test suite:

```bash
# Run all tests including NWC
./scripts/test.sh

# Run protocol package tests
go test ./pkg/protocol/...
```

### Test Coverage

Tests verify:
- Client creation and connection
- Request/response handling
- Notification subscriptions
- Error conditions and recovery
- NIP-44 encryption
- Concurrent access patterns
- Context cancellation

### Mock Testing

For testing without a real wallet:

```go
// Create mock client for testing
mockClient := nwc.NewMockClient()
mockClient.SetBalance(1000000) // 1000 sats

// Use in tests
result := make(map[string]any)
err := mockClient.Request(ctx, "get_balance", nil, &result)
```

## Examples

### Complete Payment Flow

```go
package main

import (
    "context"
    "log"
    "time"

    "git.smesh.lol/orly/pkg/protocol/nwc"
)

func main() {
    client, err := nwc.NewClient("nostr+walletconnect://...")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Get wallet info
    var info map[string]any
    if err := client.Request(ctx, "get_info", nil, &info); err != nil {
        log.Printf("Failed to get info: %v", err)
        return
    }
    log.Printf("Connected to wallet: %v", info)

    // Create invoice
    invoiceParams := map[string]any{
        "amount": 5000,  // 5 sats
        "description": "Test payment",
    }

    var invoice map[string]any
    if err := client.Request(ctx, "make_invoice", invoiceParams, &invoice); err != nil {
        log.Printf("Failed to create invoice: %v", err)
        return
    }

    bolt11 := invoice["invoice"].(string)
    log.Printf("Created invoice: %s", bolt11)

    // In a real application, you would present this invoice to the user
    // For testing, you can pay it using the same client
    payParams := map[string]any{"invoice": bolt11}
    var payment map[string]any
    if err := client.Request(ctx, "pay_invoice", payParams, &payment); err != nil {
        log.Printf("Failed to pay invoice: %v", err)
        return
    }

    log.Printf("Payment successful: %v", payment)
}
```

### Notification Handler

```go
func setupPaymentHandler(client *nwc.Client) {
    ctx := context.Background()

    err := client.SubscribeNotifications(ctx, func(notificationType string, notification map[string]any) error {
        log.Printf("Received notification: %s", notificationType)

        switch notificationType {
        case "payment_received":
            // Grant access to paid feature
            userID := notification["description"].(string)
            amount := notification["amount"].(float64)
            grantPremiumAccess(userID, amount)

        case "payment_sent":
            // Log outgoing payment
            amount := notification["amount"].(float64)
            log.Printf("Outgoing payment: %.2f sats", amount/1000)

        default:
            log.Printf("Unknown notification type: %s", notificationType)
        }

        return nil
    })

    if err != nil {
        log.Printf("Failed to subscribe to notifications: %v", err)
    }
}
```

## Development

### Building

```bash
go build ./pkg/protocol/nwc
```

### Code Quality

- Comprehensive error handling
- Go context support
- Thread-safe operations
- Extensive test coverage
- Proper resource cleanup

## Troubleshooting

### Common Issues

1. **Connection Failed**: Check NWC URI format and wallet availability
2. **Timeout Errors**: Use context with appropriate timeouts
3. **Encryption Errors**: Verify NIP-44 implementation compatibility
4. **Notification Issues**: Check wallet support for notifications

### Debugging

Enable debug logging:

```bash
export ORLY_LOG_LEVEL=debug
```

Monitor relay logs for NWC operations.

## License

Part of the git.smesh.lol/orly project. See main LICENSE file.
