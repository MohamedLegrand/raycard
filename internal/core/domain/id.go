package domain

import (
	"crypto/rand"
	"fmt"
)

// NewID génère un identifiant UUID v4 en n'utilisant que crypto/rand,
// afin que le domaine reste sans aucune dépendance externe (voir README
// hexagonal : core/domain ne doit importer que la bibliothèque standard).
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("domain: lecture aléatoire impossible: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
