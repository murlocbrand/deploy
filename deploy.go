// Copyright (c) 2016 Axel Smeets
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
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

func logTaskStatus(id int, target *targetConfig, status string) {
	log.Printf("%s task #%d (%s@%s)\n",
		status, id, target.User, target.Host)
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

	if err := target.Preprocess(); err != nil {
		logTaskStatus(taskId, &target, "Aborted: "+err.Error())
		return
	}

	conf, err := target.ClientConfig()
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
