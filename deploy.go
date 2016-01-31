// Copyright (c) 2016 Axel Smeets
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

var (
	target = flag.String("target", "target.json", "path to the `file` with JSON-formatted targets")
	script = flag.String("script", "script.sh", "path to the shell script `file`")
	stdout = flag.Bool("stdout", false, "pipe remote shell stdout to current shell stdout")
)

func fatalError(msg string, err error) {
	if err != nil {
		log.Fatal(msg + ": " + err.Error())
	}
}

func getUsername() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed getting active user: %s", err.Error())
	}

	username := current.Username
	if strings.Contains(username, "\\") {
		// probably on a windows machine: DOMAIN\USER
		username = strings.Split(username, "\\")[1]
	}
	return username, nil
}

func getHomeDir() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed getting active user: %s", err.Error())
	}
	return current.HomeDir, nil
}

func logTaskStatus(id int, target *targetConfig, status string) {
	log.Printf("%s task #%d (%s@%s)\n",
		status, id, target.User, target.Host)
}

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

// Fix the configuration before handing it to parseClientConfig:
// 	- if no username, set to current user's name
// 	- if ~ found in pki artifact, expand it to home directory
func preprocessTarget(target *targetConfig) error {
	// No user? Try to use current user's username.
	if len(target.User) == 0 {
		username, err := getUsername()
		if err != nil {
			return fmt.Errorf("failed resolving username: %s", err.Error())
		}
		target.User = username
	}

	// A ~ in the private key path? Try to expand it!
	if target.Auth.Method == "pki" &&
		strings.Contains(target.Auth.Artifact, "~") {
		home, err := getHomeDir()
		if err != nil {
			return fmt.Errorf("failed expanding ~ to home dir: %s", err.Error())
		}
		target.Auth.Artifact = strings.Replace(target.Auth.Artifact, "~", home, 1)
	}

	return nil
}

func parseClientConfig(target *targetConfig) (*ssh.ClientConfig, error) {
	conf := &ssh.ClientConfig{
		User: target.User,
	}

	// Only supports password and pki methods. Soon interactive as well?
	switch target.Auth.Method {
	case "password":
		conf.Auth = []ssh.AuthMethod{
			ssh.Password(target.Auth.Artifact),
		}
	case "pki":
		pem, err := ioutil.ReadFile(target.Auth.Artifact)
		if err != nil {
			return nil, fmt.Errorf("failed reading key: %s", err.Error())
		}

		signer, err := ssh.ParsePrivateKey(pem)
		if err != nil {
			return nil, fmt.Errorf("failed parsing key: %s", err.Error())
		}

		conf.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	default:
		err := fmt.Errorf("unknown authentication method %s", target.Auth.Method)
		return nil, err

	}

	return conf, nil
}

func execRemoteShell(host string, conf *ssh.ClientConfig, script *[]byte) error {
	client, err := ssh.Dial("tcp", host, conf)
	if err != nil {
		return fmt.Errorf("Failed to dial target: %s", err.Error())
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("Failed to start session: %s", err.Error())
	}
	defer session.Close()

	if *stdout {
		session.Stdout = os.Stdout
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("Failed setting up stdin: %s\n", err.Error())
	}

	if err := session.Shell(); err != nil {
		return fmt.Errorf("Error starting remote shell: %s\n", err.Error())
	}

	if _, err := stdin.Write(*script); err != nil {
		return fmt.Errorf("Error writing script: %s\n", err.Error())
	}

	if err := stdin.Close(); err != nil {
		return fmt.Errorf("Error closing session stdin: %s\n", err.Error())
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("Error during shell session: %s\n", err.Error())
	}

	return nil
}

func deploy(taskId int, target targetConfig, script *[]byte, wg *sync.WaitGroup) {
	defer wg.Done()

	if err := preprocessTarget(&target); err != nil {
		logTaskStatus(taskId, &target, "Aborted: "+err.Error())
		return
	}

	conf, err := parseClientConfig(&target)
	if err != nil {
		logTaskStatus(taskId, &target, "Aborted: "+err.Error())
		return
	}

	logTaskStatus(taskId, &target, "Starting")

	if err := execRemoteShell(target.Host, conf, script); err != nil {
		logTaskStatus(taskId, &target, "Errored: "+err.Error())
	} else {
		logTaskStatus(taskId, &target, "Completed")
	}
}

func main() {
	flag.Parse()

	// Easier on memory usage to use a json Decoder on the file (a reader),
	// than reading file into memory and calling Unmarshal.
	authReader, err := os.Open(*target)
	fatalError("Failed to read target config", err)
	defer authReader.Close()

	// Easier on disk to read file once, instead of once/target. (Readers are
	// consumed and must be instantiated per target)
	cmd, err := ioutil.ReadFile(*script)
	fatalError("Couldn't read script file", err)

	// Use array (as opposed to floating entries) so json is valid
	var targets []targetConfig

	authDec := json.NewDecoder(authReader)
	err = authDec.Decode(&targets)
	fatalError("Couldn't parse targets file", err)

	var wg sync.WaitGroup
	wg.Add(len(targets))
	for i, conf := range targets {
		go deploy(i, conf, &cmd, &wg)
	}

	wg.Wait()
}
