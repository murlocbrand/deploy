package main

import (
	"io/ioutil"
	"strings"
	"golang.org/x/crypto/ssh"
	"os/user"
	"fmt"
)


/*	{
 *		"username": "bob",
 *		"host": "myserver:22",
 *		"auth": {
 *			"method": "password" or "pki",
 *			"artifact": "<secret>" or "/path/to/private_key.pem"
 * 		}
 * 	}
 */
type targetConfig struct {
	User string `json:"username"`
	Host string `json:"host"`
	Auth struct {
		Method   string `json:"method"`
		Artifact string `json:"artifact"`
	} `json:"auth"`
}

// Fix the configuration before handing it to clientConfig():
// 	- if ~ found in pki artifact, expand it to home directory
func (target *targetConfig) Preprocess() error {
	// A ~ in the private key path? Try to expand it!
	if target.Auth.Method == "pki" &&
		strings.Contains(target.Auth.Artifact, "~") {
		active, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed getting current user while expanding home (~): %s", err.Error())
		}
		target.Auth.Artifact = strings.Replace(target.Auth.Artifact, "~", active.HomeDir, 1)
	}

	return nil
}

// Generate a password-auth'd ssh ClientConfig
func (target *targetConfig) password() (*ssh.ClientConfig, error) {
	// Password might be "" so can't check len(artifact)
	return &ssh.ClientConfig{
		User: target.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(target.Auth.Artifact),
		},
	}, nil
}

// Generate a PKI-auth'd ssh ClientConfig
func (target *targetConfig) pki() (*ssh.ClientConfig, error) {
	pem, err := ioutil.ReadFile(target.Auth.Artifact)
	if err != nil {
		return nil, fmt.Errorf("failed reading key: %s", err.Error())
	}

	signer, err := ssh.ParsePrivateKey(pem)
	if err != nil {
		return nil, fmt.Errorf("failed parsing key: %s", err.Error())
	}

	return &ssh.ClientConfig{
		User: target.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}, nil
}

// Figure out how to generate the ssh ClientConfig, or bail
func (target *targetConfig) ClientConfig() (*ssh.ClientConfig, error) {
	if len(target.User) == 0 {
		return nil, fmt.Errorf("target config requires a username")
	}

	// Only supports password and pki methods. Soon interactive as well?
	switch target.Auth.Method {
	case "password":
		return target.password()
	case "pki":
		return target.pki()
	default:
		err := fmt.Errorf("unknown authentication method %s", target.Auth.Method)
		return nil, err
	}
}
