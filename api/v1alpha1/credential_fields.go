// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sort"
	"strings"
)

// reservedCredentialFieldNames is an exact, normalized vocabulary for input
// fields that would create a provider-credential channel. Values are not
// inspected: task prose may legitimately discuss credentials, while actual
// provider credentials must originate inside a gateway adapter.
var reservedCredentialFieldNames = map[string]struct{}{
	"token":                        {},
	"githubtoken":                  {},
	"ghtoken":                      {},
	"ghenterprisetoken":            {},
	"githubenterprisetoken":        {},
	"npmtoken":                     {},
	"gitlabtoken":                  {},
	"sshprivatekey":                {},
	"auth":                         {},
	"authtoken":                    {},
	"accesstoken":                  {},
	"refreshtoken":                 {},
	"clientsecret":                 {},
	"azureclientsecret":            {},
	"apikey":                       {},
	"openaiapikey":                 {},
	"anthropicapikey":              {},
	"azureopenaiapikey":            {},
	"accesskey":                    {},
	"awsaccesskeyid":               {},
	"awssecretaccesskey":           {},
	"awssessiontoken":              {},
	"secretkey":                    {},
	"privatekey":                   {},
	"password":                     {},
	"secret":                       {},
	"credential":                   {},
	"credentials":                  {},
	"authorization":                {},
	"googleapplicationcredentials": {},
}

// NormalizeCredentialFieldName applies the contract's single normalization
// rule: lowercase, then remove '-' and '_'. It does not perform substring or
// fuzzy matching.
func NormalizeCredentialFieldName(name string) string {
	lowered := strings.ToLower(name)
	return strings.NewReplacer("-", "", "_", "").Replace(lowered)
}

// ReservedCredentialFieldName reports whether name is one of the exact
// credential-bearing fields excluded from public and worker-bound input.
func ReservedCredentialFieldName(name string) bool {
	_, reserved := reservedCredentialFieldNames[NormalizeCredentialFieldName(name)]
	return reserved
}

// FindReservedCredentialFieldName returns the lexically first reserved key so
// validation of Go maps has stable error output.
func FindReservedCredentialFieldName[V any](values map[string]V) (string, bool) {
	matched := make([]string, 0)
	for key := range values {
		if ReservedCredentialFieldName(key) {
			matched = append(matched, key)
		}
	}
	if len(matched) == 0 {
		return "", false
	}
	sort.Strings(matched)
	return matched[0], true
}
