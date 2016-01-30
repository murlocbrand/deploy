package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

var (
	target = flag.String("target", "target.json", "json `file` with deployment targets")
	script = flag.String("script", "script.sh", "shell script `file` with deployment procedure")
	stdout = flag.Bool("stdout", true, "should ssh session stdout be piped?")
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

func logTargetStatus(id int, target *targetConfig, status string) {
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
	if len(target.User) == 0 {
		username, err := getUsername()
		if err != nil {
			return fmt.Errorf("failed resolving username: %s", err.Error())
		}
		target.User = username
	}

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

func deploy(host string, conf *ssh.ClientConfig, script *os.File) error {
	client, err := ssh.Dial("tcp", host, conf)
	if err != nil {
		return fmt.Errorf("Failed to dial target: %s", err.Error())
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("Failed to start session: %s", err.Error())
	}
	defer session.Close()

	if *stdout {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("Failed setting up stdin: %s\n", err.Error())
	}

	session.Shell()
	io.Copy(stdin, script)
	stdin.Close()
	session.Wait()

	return nil
}

func main() {
	flag.Parse()

	authReader, err := os.Open(*target)
	fatalError("Failed to read target config", err)

	cmdReader, err := os.Open(*script)
	fatalError("Failed to read deployment script", err)

	authDec := json.NewDecoder(authReader)
	var wg sync.WaitGroup

	for i := 0; authDec.More(); i++ {

		var connfig targetConfig
		err := authDec.Decode(&connfig)
		if err != nil && err != io.EOF {
			log.Fatalln("Couldn't parse targets file:", err)
			os.Exit(1)
		}

		wg.Add(1)
		go func(id int, target *targetConfig) {
			defer wg.Done()

			err := preprocessTarget(target)
			if err != nil {
				log.Printf("[%d] %s\n", id, err.Error())
				return
			}

			conf, err := parseClientConfig(target)
			if err != nil {
				log.Printf("[%d] %s\n", id, err.Error())
				return
			}

			logTargetStatus(id, target, "Starting")

			err = deploy(connfig.Host, conf, cmdReader)

			if err != nil {
				log.Printf("[%d] %s\n", id, err.Error())
				return
			}

			logTargetStatus(id, target, "Completed")
		}(i, &connfig)
	}

	wg.Wait()
}
