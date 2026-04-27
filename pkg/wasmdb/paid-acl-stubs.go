//go:build js && wasm

package wasmdb

import (
	"fmt"

	"git.smesh.lol/orly/pkg/database"
)

var errPaidACLNotSupported = fmt.Errorf("paid ACL not supported on wasmdb backend")

func (w *W) SavePaidSubscription(sub *database.PaidSubscription) error {
	return errPaidACLNotSupported
}

func (w *W) GetPaidSubscription(pubkeyHex string) (*database.PaidSubscription, error) {
	return nil, errPaidACLNotSupported
}

func (w *W) DeletePaidSubscription(pubkeyHex string) error {
	return errPaidACLNotSupported
}

func (w *W) ListPaidSubscriptions() ([]*database.PaidSubscription, error) {
	return nil, errPaidACLNotSupported
}

func (w *W) ClaimAlias(alias, pubkeyHex string) error {
	return errPaidACLNotSupported
}

func (w *W) GetAliasByPubkey(pubkeyHex string) (string, error) {
	return "", errPaidACLNotSupported
}

func (w *W) GetAliasesByPubkey(pubkeyHex string) ([]string, error) {
	return nil, errPaidACLNotSupported
}

func (w *W) GetPubkeyByAlias(alias string) (string, error) {
	return "", errPaidACLNotSupported
}

func (w *W) IsAliasTaken(alias string) (bool, error) {
	return false, errPaidACLNotSupported
}
