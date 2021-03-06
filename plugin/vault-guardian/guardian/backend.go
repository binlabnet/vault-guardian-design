package guardian

import (
	"context"
	"fmt"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
)

// Factory returns a new backend as logical.Backend.
func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := Backend(conf)
	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}
	return b, nil
}

func Backend(c *logical.BackendConfig) *backend {
	var b backend
	b.Backend = &framework.Backend{
		Help:         "",
		PathsSpecial: &logical.Paths{Unauthenticated: []string{"login"}},
		Paths: framework.PathAppend([]*framework.Path{
			&framework.Path{
				Pattern: "login",
				Fields: map[string]*framework.FieldSchema{
					"okta_username": &framework.FieldSchema{
						Type:        framework.TypeString,
						Description: "Username of Okta account to login, probably an email address."},
					"okta_password": &framework.FieldSchema{
						Type:        framework.TypeString,
						Description: "Password for associated Okta account."},
				},
				Callbacks: map[logical.Operation]framework.OperationFunc{
					logical.UpdateOperation: b.pathLogin,
				},
			},
			&framework.Path{
				Pattern: "sign",
				Fields: map[string]*framework.FieldSchema{
					"raw_data": &framework.FieldSchema{
						Type:        framework.TypeString,
						Description: "Raw hashed transaction data to sign, do not include the initial 0x.",
					},
					"address_index": &framework.FieldSchema{
						Type:        framework.TypeInt,
						Description: "Integer index of which generated address to use.",
						Default:     0,
					},
				},
				Callbacks: map[logical.Operation]framework.OperationFunc{
					logical.CreateOperation: b.pathSign,
					logical.UpdateOperation: b.pathSign,
					logical.ReadOperation:   b.pathGetAddress,
				},
			},
			&framework.Path{
				Pattern: "authorize",
				Fields: map[string]*framework.FieldSchema{
					"secret_id": &framework.FieldSchema{
						Type:        framework.TypeString,
						Description: "SecretID of the Guardian AppRole.",
					},
					"okta_url": &framework.FieldSchema{
						Type:        framework.TypeString,
						Description: "Organization's Okta URL.",
					},
					"okta_token": &framework.FieldSchema{
						Type:        framework.TypeString,
						Description: "Permissioned API token from Okta organization.",
					},
				},
				Callbacks: map[logical.Operation]framework.OperationFunc{
					logical.CreateOperation: b.pathAuthorize,
					logical.UpdateOperation: b.pathAuthorize,
				},
			},
		}),
		BackendType: logical.TypeLogical,
	}
	return &b
}

type backend struct {
	*framework.Backend
}

func (b *backend) Config(ctx context.Context, s logical.Storage) (*Config, error) {
	config, err := s.Get(ctx, "config")
	if err != nil {
		return nil, err
	}
	var result Config
	if config != nil {
		if err := config.DecodeJSON(&result); err != nil {
			return nil, err
		}
	} else {
		result = Config{"", "", ""}
	}
	return &result, nil
}

func (b *backend) pathExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	out, err := req.Storage.Get(ctx, req.Path)
	if err != nil {
		return false, fmt.Errorf("existence check failed: %v", err)
	}

	return out != nil, nil
}
