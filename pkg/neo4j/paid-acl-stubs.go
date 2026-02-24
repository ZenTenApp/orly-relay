package neo4j

import (
	"fmt"

	"next.orly.dev/pkg/database"
)

var errPaidACLNotSupported = fmt.Errorf("paid ACL not supported on Neo4j backend")

func (n *N) SavePaidSubscription(sub *database.PaidSubscription) error {
	return errPaidACLNotSupported
}

func (n *N) GetPaidSubscription(pubkeyHex string) (*database.PaidSubscription, error) {
	return nil, errPaidACLNotSupported
}

func (n *N) DeletePaidSubscription(pubkeyHex string) error {
	return errPaidACLNotSupported
}

func (n *N) ListPaidSubscriptions() ([]*database.PaidSubscription, error) {
	return nil, errPaidACLNotSupported
}

func (n *N) ClaimAlias(alias, pubkeyHex string) error {
	return errPaidACLNotSupported
}

func (n *N) GetAliasByPubkey(pubkeyHex string) (string, error) {
	return "", errPaidACLNotSupported
}

func (n *N) GetPubkeyByAlias(alias string) (string, error) {
	return "", errPaidACLNotSupported
}

func (n *N) IsAliasTaken(alias string) (bool, error) {
	return false, errPaidACLNotSupported
}
