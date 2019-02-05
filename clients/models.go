/*
 * Copyright 2019 Kopano and its licensors
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, version 3,
 * as published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package clients

import (
	"fmt"

	"github.com/mendsley/gojwk"
	_ "gopkg.in/yaml.v2" // Make sure we have yaml.
)

// RegistryData is the base structur of our client registry configuration file.
type RegistryData struct {
	Clients []*ClientRegistration `yaml:"clients,flow"`
}

// ClientRegistration defines a client with its properties.
type ClientRegistration struct {
	ID              string `yaml:"id"`
	Secret          string `yaml:"secret"`
	Name            string `yaml:"name"`
	ApplicationType string `yaml:"application_type"`

	Trusted       bool     `yaml:"trusted"`
	TrustedScopes []string `yaml:"trusted_scopes"`
	Insecure      bool     `yaml:"insecure"`

	Origins []string `yaml:"origins,flow"`

	JWKS                       *gojwk.Key `yaml:"jwks"`
	RawRequestObjectSigningAlg string     `yaml:"request_object_signing_alg"`
}

// Secure looks up the a matching key from the accociated client registration
// and returns its public key part as a secured client.
func (cr *ClientRegistration) Secure(rawKid interface{}) (*Secured, error) {
	k, err := cr.findKeyByKid(rawKid)
	if err != nil {
		return nil, err
	}

	pubKey, err := k.DecodePublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	return &Secured{
		ID:              cr.ID,
		DisplayName:     cr.Name,
		ApplicationType: cr.ApplicationType,

		Kid:       k.Kid,
		PublicKey: pubKey,

		TrustedScopes: cr.TrustedScopes,
	}, nil
}

// Private looks up a matching key from the accociated client registration and
// returns its private and publuc key parts as secured client.
func (cr *ClientRegistration) Private(rawKid interface{}) (*Secured, error) {
	k, err := cr.findKeyByKid(rawKid)
	if err != nil {
		return nil, err
	}

	privKey, err := k.DecodePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %v", err)
	}
	pubKey, err := k.DecodePublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	return &Secured{
		ID:              cr.ID,
		DisplayName:     cr.Name,
		ApplicationType: cr.ApplicationType,

		Kid:        k.Kid,
		PublicKey:  pubKey,
		PrivateKey: privKey,

		TrustedScopes: cr.TrustedScopes,
	}, nil
}

func (cr *ClientRegistration) findKeyByKid(rawKid interface{}) (*gojwk.Key, error) {
	var key *gojwk.Key

	switch len(cr.JWKS.Keys) {
	case 0:
		// breaks
	case 1:
		// Use the one and only, no matter what kid says.
		key = cr.JWKS.Keys[0]
	default:
		// Find by kid.
		kid, _ := rawKid.(string)
		if kid == "" {
			kid = "default"
		}
		for _, k := range cr.JWKS.Keys {
			if kid == k.Kid {
				key = k
				break
			}
		}
	}

	if key == nil {
		return nil, fmt.Errorf("unknown kid")
	}
	if key.Use != "sig" {
		return nil, fmt.Errorf("unsupported key use value - must be sig")
	}
	return key, nil
}
