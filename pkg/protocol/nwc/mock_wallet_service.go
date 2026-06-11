package nwc

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/crypto/encryption"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

// --- actor request types ---

type mwsGetBalanceReq struct {
	resp chan int64
}

type mwsDebitBalanceReq struct {
	amount int64
	resp   chan bool // true if sufficient
}

type mwsCreditBalanceReq struct {
	amount int64
}

type mwsGetOrCreateConvKeyReq struct {
	clientPubkeyHex string
	clientPubkey    []byte
	resp            chan mwsGetOrCreateConvKeyResp
}

type mwsGetOrCreateConvKeyResp struct {
	key []byte
	err error
}

type mwsGetAllClientsReq struct {
	resp chan map[string][]byte
}

// MockWalletService implements a mock NIP-47 wallet service for testing
type MockWalletService struct {
	relay           string
	walletSecretKey signer.I
	walletPublicKey []byte
	client          *ws.Client
	ctx             context.Context
	cancel          context.CancelFunc

	// actor channels
	getBalanceCh          chan mwsGetBalanceReq
	debitBalanceCh        chan mwsDebitBalanceReq
	creditBalanceCh       chan mwsCreditBalanceReq
	getOrCreateConvKeyCh  chan mwsGetOrCreateConvKeyReq
	getAllClientsCh       chan mwsGetAllClientsReq

	stop chan struct{}
	done chan struct{}
}

// NewMockWalletService creates a new mock wallet service
func NewMockWalletService(
	relay string, initialBalance int64,
) (service *MockWalletService, err error) {
	var walletKey *p8k.Signer
	if walletKey, err = p8k.New(); chk.E(err) {
		return
	}
	if err = walletKey.Generate(); chk.E(err) {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	service = &MockWalletService{
		relay:           relay,
		walletSecretKey: walletKey,
		walletPublicKey: walletKey.Pub(),
		ctx:             ctx,
		cancel:          cancel,

		getBalanceCh:         make(chan mwsGetBalanceReq),
		debitBalanceCh:       make(chan mwsDebitBalanceReq),
		creditBalanceCh:      make(chan mwsCreditBalanceReq, 16),
		getOrCreateConvKeyCh: make(chan mwsGetOrCreateConvKeyReq),
		getAllClientsCh:      make(chan mwsGetAllClientsReq),

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go service.runActor(initialBalance)
	return
}

func (m *MockWalletService) runActor(initialBalance int64) {
	defer close(m.done)

	balance := initialBalance
	connectedClients := make(map[string][]byte) // pubkey hex -> conversation key

	for {
		select {
		case <-m.stop:
			return
		case req := <-m.getBalanceCh:
			req.resp <- balance
		case req := <-m.debitBalanceCh:
			if balance*1000 < req.amount {
				req.resp <- false
			} else {
				balance -= req.amount / 1000
				req.resp <- true
			}
		case req := <-m.creditBalanceCh:
			balance += req.amount / 1000
		case req := <-m.getOrCreateConvKeyCh:
			if existingKey, exists := connectedClients[req.clientPubkeyHex]; exists {
				req.resp <- mwsGetOrCreateConvKeyResp{key: existingKey}
			} else {
				conversationKey, err := encryption.GenerateConversationKey(
					m.walletSecretKey.Sec(), req.clientPubkey,
				)
				if err != nil {
					req.resp <- mwsGetOrCreateConvKeyResp{err: err}
				} else {
					connectedClients[req.clientPubkeyHex] = conversationKey
					req.resp <- mwsGetOrCreateConvKeyResp{key: conversationKey}
				}
			}
		case req := <-m.getAllClientsCh:
			result := make(map[string][]byte, len(connectedClients))
			for k, v := range connectedClients {
				result[k] = v
			}
			req.resp <- result
		}
	}
}

// Start begins the mock wallet service
func (m *MockWalletService) Start() (err error) {
	if m.client, err = ws.RelayConnect(m.ctx, m.relay); chk.E(err) {
		return fmt.Errorf("failed to connect to relay: %w", err)
	}

	if err = m.publishWalletInfo(); chk.E(err) {
		return fmt.Errorf("failed to publish wallet info: %w", err)
	}

	if err = m.subscribeToRequests(); chk.E(err) {
		return fmt.Errorf("failed to subscribe to requests: %w", err)
	}

	return
}

// Stop stops the mock wallet service
func (m *MockWalletService) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.client != nil {
		m.client.Close()
	}
	close(m.stop)
	<-m.done
}

// GetWalletPublicKey returns the wallet's public key
func (m *MockWalletService) GetWalletPublicKey() []byte {
	return m.walletPublicKey
}

// publishWalletInfo publishes the NIP-47 info event (kind 13194)
func (m *MockWalletService) publishWalletInfo() (err error) {
	capabilities := []string{
		"get_info",
		"get_balance",
		"make_invoice",
		"pay_invoice",
	}

	info := map[string]any{
		"capabilities":  capabilities,
		"notifications": []string{"payment_received", "payment_sent"},
	}

	var content []byte
	if content, err = json.Marshal(info); chk.E(err) {
		return
	}

	ev := &event.E{
		Content:   content,
		CreatedAt: time.Now().Unix(),
		Kind:      13194,
		Tags:      tag.NewS(),
	}

	if err = ev.Sign(m.walletSecretKey); chk.E(err) {
		return
	}

	return m.client.Publish(m.ctx, ev)
}

// subscribeToRequests subscribes to NWC request events (kind 23194)
func (m *MockWalletService) subscribeToRequests() (err error) {
	var sub *ws.Subscription
	if sub, err = m.client.Subscribe(
		m.ctx, filter.NewS(
			&filter.F{
				Kinds: kind.NewS(kind.New(23194)),
				Tags: tag.NewS(
					tag.NewFromAny("p", hex.Enc(m.walletPublicKey)),
				),
				Since: &timestamp.T{V: time.Now().Unix()},
			},
		),
	); chk.E(err) {
		return
	}

	go m.handleRequestEvents(sub)
	return
}

// handleRequestEvents processes incoming NWC request events
func (m *MockWalletService) handleRequestEvents(sub *ws.Subscription) {
	for {
		select {
		case <-m.ctx.Done():
			return
		case ev := <-sub.Events:
			if ev == nil {
				continue
			}
			if err := m.processRequestEvent(ev); chk.E(err) {
				fmt.Printf("Error processing request event: %v\n", err)
			}
		}
	}
}

// processRequestEvent processes a single NWC request event
func (m *MockWalletService) processRequestEvent(ev *event.E) (err error) {
	clientPubkey := ev.Pubkey
	clientPubkeyHex := hex.Enc(clientPubkey)

	// Get or create conversation key via actor
	resp := make(chan mwsGetOrCreateConvKeyResp, 1)
	select {
	case m.getOrCreateConvKeyCh <- mwsGetOrCreateConvKeyReq{
		clientPubkeyHex: clientPubkeyHex,
		clientPubkey:    clientPubkey,
		resp:            resp,
	}:
	case <-m.stop:
		return fmt.Errorf("service stopped")
	}

	r := <-resp
	if r.err != nil {
		return r.err
	}
	conversationKey := r.key

	var decrypted string
	if decrypted, err = encryption.Decrypt(
		conversationKey, string(ev.Content),
	); chk.E(err) {
		return
	}

	var request map[string]any
	if err = json.Unmarshal([]byte(decrypted), &request); chk.E(err) {
		return
	}

	method, ok := request["method"].(string)
	if !ok {
		return fmt.Errorf("invalid method")
	}

	params := request["params"]

	var result any
	if result, err = m.processMethod(method, params); chk.E(err) {
		return m.sendErrorResponse(
			clientPubkey, conversationKey, "INTERNAL", err.Error(),
		)
	}

	return m.sendSuccessResponse(clientPubkey, conversationKey, result)
}

// processMethod handles the actual NWC method execution
func (m *MockWalletService) processMethod(
	method string, params any,
) (result any, err error) {
	switch method {
	case "get_info":
		return m.getInfo()
	case "get_balance":
		return m.getBalance()
	case "make_invoice":
		return m.makeInvoice(params)
	case "pay_invoice":
		return m.payInvoice(params)
	default:
		err = fmt.Errorf("unsupported method: %s", method)
		return
	}
}

// getInfo returns wallet information
func (m *MockWalletService) getInfo() (result map[string]any, err error) {
	result = map[string]any{
		"alias":        "Mock Wallet",
		"color":        "#3399FF",
		"pubkey":       hex.Enc(m.walletPublicKey),
		"network":      "mainnet",
		"block_height": 850000,
		"block_hash":   "0000000000000000000123456789abcdef",
		"methods": []string{
			"get_info", "get_balance", "make_invoice", "pay_invoice",
		},
	}
	return
}

// getBalance returns the current wallet balance
func (m *MockWalletService) getBalance() (result map[string]any, err error) {
	resp := make(chan int64, 1)
	select {
	case m.getBalanceCh <- mwsGetBalanceReq{resp: resp}:
		balance := <-resp
		result = map[string]any{
			"balance": balance * 1000, // convert to msats
		}
	case <-m.stop:
		err = fmt.Errorf("service stopped")
	}
	return
}

// makeInvoice creates a Lightning invoice
func (m *MockWalletService) makeInvoice(params any) (
	result map[string]any, err error,
) {
	paramsMap, ok := params.(map[string]any)
	if !ok {
		err = fmt.Errorf("invalid params")
		return
	}

	amount, ok := paramsMap["amount"].(float64)
	if !ok {
		err = fmt.Errorf("missing or invalid amount")
		return
	}

	description := ""
	if desc, ok := paramsMap["description"].(string); ok {
		description = desc
	}

	paymentHash := make([]byte, 32)
	rand.Read(paymentHash)

	bolt11 := fmt.Sprintf("lnbc%dm1pwxxxxxxx", int64(amount/1000))

	result = map[string]any{
		"type":         "incoming",
		"invoice":      bolt11,
		"description":  description,
		"payment_hash": hex.Enc(paymentHash),
		"amount":       int64(amount),
		"created_at":   time.Now().Unix(),
		"expires_at":   time.Now().Add(24 * time.Hour).Unix(),
	}
	return
}

// payInvoice pays a Lightning invoice
func (m *MockWalletService) payInvoice(params any) (
	result map[string]any, err error,
) {
	paramsMap, ok := params.(map[string]any)
	if !ok {
		err = fmt.Errorf("invalid params")
		return
	}

	invoice, ok := paramsMap["invoice"].(string)
	if !ok {
		err = fmt.Errorf("missing or invalid invoice")
		return
	}

	amount := int64(1000) // 1000 msats

	// Check and debit balance via actor
	resp := make(chan bool, 1)
	select {
	case m.debitBalanceCh <- mwsDebitBalanceReq{amount: amount, resp: resp}:
		if !<-resp {
			err = fmt.Errorf("insufficient balance")
			return
		}
	case <-m.stop:
		err = fmt.Errorf("service stopped")
		return
	}

	preimage := make([]byte, 32)
	rand.Read(preimage)

	result = map[string]any{
		"type":       "outgoing",
		"invoice":    invoice,
		"amount":     amount,
		"preimage":   hex.Enc(preimage),
		"created_at": time.Now().Unix(),
	}

	// Emit payment_sent notification
	go m.emitPaymentNotification("payment_sent", result)
	return
}

// sendSuccessResponse sends a successful NWC response
func (m *MockWalletService) sendSuccessResponse(
	clientPubkey []byte, conversationKey []byte, result any,
) (err error) {
	response := map[string]any{
		"result": result,
	}

	var responseBytes []byte
	if responseBytes, err = json.Marshal(response); chk.E(err) {
		return
	}

	return m.sendEncryptedResponse(clientPubkey, conversationKey, responseBytes)
}

// sendErrorResponse sends an error NWC response
func (m *MockWalletService) sendErrorResponse(
	clientPubkey []byte, conversationKey []byte, code, message string,
) (err error) {
	response := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}

	var responseBytes []byte
	if responseBytes, err = json.Marshal(response); chk.E(err) {
		return
	}

	return m.sendEncryptedResponse(clientPubkey, conversationKey, responseBytes)
}

// sendEncryptedResponse sends an encrypted response event (kind 23195)
func (m *MockWalletService) sendEncryptedResponse(
	clientPubkey []byte, conversationKey []byte, content []byte,
) (err error) {
	var encrypted string
	if encrypted, err = encryption.Encrypt(
		conversationKey, content, nil,
	); chk.E(err) {
		return
	}

	ev := &event.E{
		Content:   []byte(encrypted),
		CreatedAt: time.Now().Unix(),
		Kind:      23195,
		Tags: tag.NewS(
			tag.NewFromAny("encryption", "nip44_v2"),
			tag.NewFromAny("p", hex.Enc(clientPubkey)),
		),
	}

	if err = ev.Sign(m.walletSecretKey); chk.E(err) {
		return
	}

	return m.client.Publish(m.ctx, ev)
}

// emitPaymentNotification emits a payment notification (kind 23197)
func (m *MockWalletService) emitPaymentNotification(
	notificationType string, paymentData map[string]any,
) (err error) {
	notification := map[string]any{
		"notification_type": notificationType,
		"notification":      paymentData,
	}

	var content []byte
	if content, err = json.Marshal(notification); chk.E(err) {
		return
	}

	// Get all connected clients via actor
	resp := make(chan map[string][]byte, 1)
	select {
	case m.getAllClientsCh <- mwsGetAllClientsReq{resp: resp}:
	case <-m.stop:
		return
	}
	clients := <-resp

	for clientPubkeyHex, conversationKey := range clients {
		var clientPubkey []byte
		if clientPubkey, err = hex.Dec(clientPubkeyHex); chk.E(err) {
			continue
		}

		var encrypted string
		if encrypted, err = encryption.Encrypt(
			conversationKey, content, nil,
		); chk.E(err) {
			continue
		}

		ev := &event.E{
			Content:   []byte(encrypted),
			CreatedAt: time.Now().Unix(),
			Kind:      23197,
			Tags: tag.NewS(
				tag.NewFromAny("encryption", "nip44_v2"),
				tag.NewFromAny("p", hex.Enc(clientPubkey)),
			),
		}

		if err = ev.Sign(m.walletSecretKey); chk.E(err) {
			continue
		}

		m.client.Publish(m.ctx, ev)
	}
	return
}

// SimulateIncomingPayment simulates an incoming payment for testing
func (m *MockWalletService) SimulateIncomingPayment(
	pubkey []byte, amount int64, description string,
) (err error) {
	// Credit balance via actor
	select {
	case m.creditBalanceCh <- mwsCreditBalanceReq{amount: amount}:
	case <-m.stop:
		return fmt.Errorf("service stopped")
	}

	paymentHash := make([]byte, 32)
	rand.Read(paymentHash)

	preimage := make([]byte, 32)
	rand.Read(preimage)

	paymentData := map[string]any{
		"type":         "incoming",
		"invoice":      fmt.Sprintf("lnbc%dm1pwxxxxxxx", amount/1000),
		"description":  description,
		"amount":       amount,
		"payment_hash": hex.Enc(paymentHash),
		"preimage":     hex.Enc(preimage),
		"created_at":   time.Now().Unix(),
	}

	return m.emitPaymentNotification("payment_received", paymentData)
}
