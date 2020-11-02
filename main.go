// Copyright 2020 Tero Saarni
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/tsaarni/11th-init/internal/pkg/elasticwriter"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-server HOST:PORT] [-cert FILE] [-key FILE] [-ca-cert FILE] -- command [ARGS]\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	var params struct {
		server string
		cert   string
		key    string
		ca     string
	}

	flag.StringVar(&params.server, "server", "", "Server address to stream logs with")
	flag.StringVar(&params.cert, "cert", "", "Client certificate")
	flag.StringVar(&params.key, "key", "", "Private key associated with the client certificate")
	flag.StringVar(&params.ca, "ca-cert", "", "CA certificate for validating server certificate")
	flag.Parse()

	// Command to be executed in child process, including its arguments.
	command := flag.Args()

	// Create channel to receive OS signals.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs)

	// Create pipe to capture stdout from child process.
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}

	// Create pipe to capture stderr from child process.
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		log.Fatal(err)
	}

	// Fork and exec the command.
	log.Println("Running:", strings.Join(command, " "))
	pid, err := os.StartProcess(command[0], command,
		&os.ProcAttr{
			Env:   os.Environ(),
			Files: []*os.File{nil, stdoutWrite, stderrWrite},
		})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Child pid:", pid.Pid)

	// The write-ends of the pipes are used in the child process, close them here in the parent process.
	stdoutWrite.Close()
	stderrWrite.Close()

	// Create log forwarder to send logs to Elasticsearch.
	elasticWriter, err := elasticwriter.New(params.server, params.cert, params.key, params.ca, 10)
	if err != nil {
		log.Fatal(err)
	}

	// Create go routines that read stdout/stderr from child and duplicates the output to both
	// stdout/stderr of parent process and Elasticsearch
	stdoutWriter := io.MultiWriter(os.Stdout, elasticWriter)
	stderrWriter := io.MultiWriter(os.Stderr, elasticWriter)

	var ioForwarders sync.WaitGroup
	ioForwarders.Add(2)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		io.Copy(stdoutWriter, stdoutRead)
	}(&ioForwarders)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		io.Copy(stderrWriter, stderrRead)
	}(&ioForwarders)

	// Wait for signals received by the parent process
	// If SIGCHLD is received then quit.
	// If other signals are received, forward them to child.
	// or at least
	//   - SIGINT
	//   - SIGHUP
	//   - SIGQUIT
	//   - SIGTERM
	var exitStatus int
	for s := range sigs {
		if s == syscall.SIGCHLD {
			var wstatus syscall.WaitStatus
			syscall.Wait4(-1, &wstatus, 0, nil)
			if wstatus.Signaled() {
				log.Println("Child received signal", wstatus.Signal())
			}
			log.Println("Child exited with status:", wstatus.ExitStatus())
			exitStatus = wstatus.ExitStatus()
			break
		}

		log.Printf("Forwarding signal %s(%d) to child pid %d", s, s, pid.Pid)
		syscall.Kill(pid.Pid, s.(syscall.Signal))
	}
	stdoutRead.Close()
	stderrRead.Close()

	// Wait for log forwarders to finish
	ioForwarders.Wait()
	elasticWriter.Close()

	os.Exit(exitStatus)
}
