package guardian

import (
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/api"
	"github.com/okta/okta-sdk-golang/okta"
)

//-----------------------------------------
//  Core Configuration
//-----------------------------------------

type GuardianClient struct {
	vault: *api.Client
	okta: *okta.Client
}

func GuardianClient(cfg GuardianConfig) *GuardianClient {
	var gc GuardianClient

	// Set up Vault client with default token
	client, err = &api.NewClient()
	client.SetToken(cfg.guardianToken)
	gc.vault = client

	// Set up Okta client
	oktaConfig := okta.NewConfig()
		.WithOrgUrl(cfg.oktaUrl)
		.WithToken(cfg.oktaToken)
	oktaClient := okta.NewClient(oktaConfig, nil, nil)
	gc.okta = oktaClient
	return &gc, nil
}

type GuardianConfig struct {
	guardianToken 	string	`json:guardian_token`
	oktaUrl 		string	`json:okta_url`
	oktaToken 		string	`json:okta_token`
}

func (cfg *GuardianConfig) Client() (GuardianClient, err) { return GuardianClient(cfg) }

func (gc *GuardianClient) pluginAuthorized() isAuthorized:bool {
	return gc.vault.Token() != ""
}

//-----------------------------------------
//  Token Operations
//-----------------------------------------

func (gc *GuardianClient) tokenFromSecretID(secret_id string) success:bool {
	authData := map[string]interface{}{
		"role_id" : "guardian-role-id",
		"secret_id" : secret_id
	}
	resp, err := gc.vault.Logical().Write("auth/approle/login", authData)
	if err != nil {
        return err
    }
    if resp.Auth == nil {
        return fmt.Errorf("no auth info returned")
	}
	return resp.Auth.ClientToken;
}

func (gc *GuardianClient) usernameFromToken(client_token string) username:string {
	resp, err := gc.vault.Logical().Write("/auth/token/lookup", map[string]interface{}{
		"token" : client_token
	})
	if err != nil {
		return nil, err
	}
	// TODO: How does this look in errors?  Is Write the correct method?
	return resp.Data.meta.username
}

func (gc *GuardianClient) readKeyHexByToken(client_token string) privKeyHex string {
	username := gc.usernameFromToken(client_token)
	resp, err := gc.vault.Logical().Read(fmt.Sprintf("/secrets/%s", username))
	if err != nil {
		return nil, err
	}
	return resp.data.privKeyHex
}

//-----------------------------------------
//  User Management
//-----------------------------------------

func (gc *GuardianClient) loginEnduser(username string, password string) (client_token string) {
	emptyClient, makeClientErr := &api.NewClient()
	if makeClientErr != nil {
		return nil, makeClientErr
	}
	loginResp, loginErr := emptyClient.Logical().Write(fmt.Sprintf("auth/okta/login/%s",username), map[string]interface{}{
		"password" : password
	})
	if loginErr != nil {
		return nil, loginErr
	}
	return loginResp.Auth.client_token
}

func (gc *GuardianClient) enduserExists(username string) username:string {
	resp, err := gc.vault.Logical().Read(fmt.Sprintf("/auth/okta/users/%s", username))
	if err != nil {
		return nil, err
	}
	// Determine what above looks like when no account is registered
	return resp
}

func (gc *GuardianClient) createEnduser(username string) publicAddressHex string, err error {
	createData := map[string]interface{}{
		"username": username,
		"policies": [],
		"groups": ["guardian-enduser"]
	}
	userResp, userErr := gc.vault.Logical().Write(fmt.Sprintf("/auth/okta/users/%s", username), createData)
	if userErr != nil {
		return nil, userErr
	}
	privKeyHex, publicAddressHex := CreateKey()
	secretData := map[string]interface{}{
		"privKeyHex" : privKeyHex
		"publicAddressHex" : publicAddressHex
	}
	keyResp, keyErr := gc.vault.Logical().Create(fmt.Sprintf("/secrets/%s", username))
	if keyErr != nil {
		return nil, keyErr
	}
	return publicAddressHex, nil
}

//-----------------------------------------
//  Okta Calls
//-----------------------------------------

func (gc *GuardianClient) oktaAccountExists(username string) exists:bool {
	// Determine what the response looks like for non-existent users
	user, resp, err := gc.okta.User.GetUser(username, nil)
	return user != nil
}