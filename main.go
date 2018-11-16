package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshData struct {
	hostname   string
	username   string
	port       int
	privatekey string
}

func loadHostFile(hostnamesfile *string, sshHostInfo map[string]sshData) {
	data, err := ioutil.ReadFile(*hostnamesfile)
	if err != nil {
		log.Fatal(err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if len(line) < 1 || strings.HasPrefix(line, "#") {
			continue
		}
		connectString := strings.Split(line, "@")
		myhostname := ""
		if len(connectString) == 1 {
			myhostname = connectString[0]
		} else {
			myhostname = connectString[1]
		}

		sshHostInfo[myhostname] = sshData{myhostname, "", 22, ""}
	}
}

//PublicKeyFile Load private key from file
func PublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func runCommand(mysshdata sshData, ch chan<- string) {

	start := time.Now()
	sshConfig := &ssh.ClientConfig{
		User: mysshdata.username,
		Auth: []ssh.AuthMethod{
			PublicKeyFile(mysshdata.privatekey),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	connection, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", mysshdata.hostname, mysshdata.port), sshConfig)
	if err != nil {
		ch <- fmt.Sprint(err)
		return
	}

	session, err := connection.NewSession()
	if err != nil {
		ch <- fmt.Sprint(err)
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		session.Close()
		ch <- fmt.Sprint(err)
		return
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		ch <- fmt.Sprint(err)
		return
	}
	go io.Copy(stdin, os.Stdin)

	stdout, err := session.StdoutPipe()
	if err != nil {
		ch <- fmt.Sprint(err)
		return
	}
	go io.Copy(os.Stdout, stdout)

	stderr, err := session.StderrPipe()
	if err != nil {
		ch <- fmt.Sprint(err)
		return
	}
	go io.Copy(os.Stderr, stderr)

	err = session.Run("echo $HOSTNAME")

	ch <- fmt.Sprintf("%.2fs %s elapsed\n", time.Since(start).Seconds(), mysshdata.hostname)
}

func main() {

	start := time.Now()
	myUser, err := user.Current()
	if err != nil {
		log.Println("Error determining current username")
		log.Fatal(err)
	}

	sshHostInfo := make(map[string]sshData)
	ch := make(chan string)

	hostnamesfile := flag.String("hosts-file", "", "File containing hostnames")
	concurCount := flag.Int("concurrent", 50, "Number of parallel connections to make")
	remoteCommand := flag.String("remote-cmd", "echo $HOSTNAME", "Number of parallel connections to make")
	sshUser := flag.String("l", myUser.Username, "SSH Username")
	sshKey := flag.String("i", fmt.Sprintf("%s/.ssh/id_rsa", myUser.HomeDir), "Private key file to use")
	flag.Parse()

	if *hostnamesfile == "" {
		log.Fatalln("You need to provide a hostnamesfile!!!")
	}

	log.SetPrefix("pssh:")
	log.Println("Starting ....")
	log.Printf("SSH Client example: ssh -i %s -p 22 %s@<hostname>", *sshKey, *sshUser)
	log.Printf("Loading hostnames file: %s", *hostnamesfile)
	loadHostFile(hostnamesfile, sshHostInfo)
	log.Printf("Loaded %d hosts", len(sshHostInfo))
	log.Printf("Concurrency count: %d", *concurCount)
	log.Printf("Remote Command: %s", *remoteCommand)

	for _, mysshData := range sshHostInfo {
		if mysshData.username == "" {
			mysshData.username = *sshUser
			mysshData.privatekey = *sshKey
			mysshData.port = 22
		}

		go runCommand(mysshData, ch)
	}

	for range sshHostInfo {
		log.Printf(<-ch)
	}

	log.Printf("%.2fs Overall elapsed time\n", time.Since(start).Seconds())
}
