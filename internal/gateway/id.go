// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
)

// IDSource issues trusted invocation identities. The gateway owns the source;
// request content cannot influence the issued value.
type IDSource func() string

// RandomIDSource returns a source producing unpredictable invocation IDs.
func RandomIDSource() IDSource {
	return func() string {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			// crypto/rand failure is unrecoverable for identity issuance.
			panic(fmt.Sprintf("gateway: invocation id entropy unavailable: %v", err))
		}
		return "inv-" + hex.EncodeToString(b[:])
	}
}

// SequenceIDSource returns a deterministic source for tests.
func SequenceIDSource(prefix string) IDSource {
	var n atomic.Uint64
	return func() string {
		return fmt.Sprintf("%s-%d", prefix, n.Add(1))
	}
}
