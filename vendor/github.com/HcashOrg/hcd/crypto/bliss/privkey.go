package bliss

import (
	hccrypto "github.com/HcashOrg/hcd/crypto"
	"github.com/HcashOrg/bliss"
)

type PrivateKey struct {
	hccrypto.PrivateKeyAdapter
	bliss.PrivateKey
}

// Public returns the PublicKey corresponding to this private key.
func (p PrivateKey) PublicKey() hccrypto.PublicKey {
	blissPkp := p.PrivateKey.PublicKey()
	pk := &PublicKey{
		PublicKey: *blissPkp,
	}
	return pk
}

// GetType satisfies the bliss PrivateKey interface.
func (p PrivateKey) GetType() int {
	return pqcTypeBliss
}

func (p PrivateKey) Serialize() []byte {
	return p.PrivateKey.Serialize()
}
