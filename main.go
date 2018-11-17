package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os/user"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshData struct {
	hostname   string
	username   string
	port       int
	privatekey string
}

type sshClient struct {
	sshData
	timeout    int32
	runCommand string
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
		myusername := ""
		if len(connectString) == 1 {
			myhostname = connectString[0]
		} else {
			myusername = connectString[0]
			myhostname = connectString[1]
		}

		sshHostInfo[myhostname] = sshData{myhostname, myusername, 22, ""}
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

//saveResults dumps command output to file in /tmp/
func saveResults(hostname string, output []byte) bool {
	timestamp := time.Now().Unix()

	err := ioutil.WriteFile(fmt.Sprintf("/tmp/%s-%d.log", hostname, timestamp), output, 0644)
	if err != nil {
		return false
	}
	return true
}

func runCommand(sshclient sshClient) (string, bool) {

	myStatus := false
	start := time.Now()
	sshConfig := &ssh.ClientConfig{
		User: sshclient.username,
		Auth: []ssh.AuthMethod{
			PublicKeyFile(sshclient.privatekey),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: time.Duration(rand.Int31n(sshclient.timeout)) * time.Millisecond,
	}

	connection, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", sshclient.hostname, sshclient.port), sshConfig)
	if err != nil {
		return fmt.Sprintf("%s : %s", sshclient.hostname, err), myStatus
	}

	session, err := connection.NewSession()
	if err != nil {
		return fmt.Sprintf("%s : %s", sshclient.hostname, err), myStatus
	}

	out, err := session.CombinedOutput(sshclient.runCommand)
	session.Close()
	connection.Close()

	if err == nil {
		myStatus = true
	}

	saveStatus := saveResults(sshclient.hostname, out)
	return fmt.Sprintf("%.2fs elapsed Host: %s : CommandSuccess: %t : output-saved: %t\n", time.Since(start).Seconds(), sshclient.hostname, myStatus, saveStatus), myStatus
}

func printUpdates(ch <-chan string, chShutDown <-chan bool) {

	chShutDownClosed := false

	for {
		if chShutDownClosed {
			return
		}
		select {
		case logUpdate, logUpdateOk := <-ch:
			if logUpdateOk {
				log.Printf(logUpdate)
			}
		case myChannelData, myChannelOpen := <-chShutDown:
			chShutDownClosed = true
			if myChannelData {
				log.Println("Completed all hosts")
			}
			if !myChannelOpen {
				log.Println("Channel closed: No more updates")
			}
		}
	}

}

func worker(ch chan<- string, chStatus chan<- bool, chsshClient <-chan sshClient, wg *sync.WaitGroup) {
	defer wg.Done()

	for sshclient := range chsshClient {
		cmdLog, cmdStatus := runCommand(sshclient)
		chStatus <- cmdStatus
		ch <- cmdLog
	}
}

func main() {

	wg := new(sync.WaitGroup)
	start := time.Now()
	sshHostInfo := make(map[string]sshData)

	myUser, err := user.Current()
	if err != nil {
		log.Println("Error determining current username")
		log.Fatal(err)
	}

	hostnamesfile := flag.String("hosts-file", "", "File containing hostnames")
	conCurMax := flag.Int("curmax", 50, "Number of parallel connections to make")
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
	log.Printf("Concurrency count: %d", *conCurMax)
	log.Printf("Remote Command: %s", *remoteCommand)

	ch := make(chan string)
	chShutDown := make(chan bool)
	chStatus := make(chan bool, len(sshHostInfo))
	chsshClient := make(chan sshClient, len(sshHostInfo))

	go printUpdates(ch, chShutDown)

	for _, mysshData := range sshHostInfo {
		if mysshData.username == "" {
			mysshData.username = *sshUser
			mysshData.privatekey = *sshKey
		}

		chsshClient <- sshClient{mysshData, 5000, *remoteCommand}
	}

	close(chsshClient)

	for i := 0; i < *conCurMax; i++ {
		wg.Add(1)
		go worker(ch, chStatus, chsshClient, wg)
	}

	//Wait for all work goroutines to finish
	wg.Wait()
	//Shutdown channel, stopping the printUpdates goroutine
	chShutDown <- true

	successResults := 0
	failedResults := 0
	close(chStatus)
	for result := range chStatus {
		if result {
			successResults++
		} else {
			failedResults++
		}
	}

	failedHosts := len(sshHostInfo) - successResults - failedResults
	log.Printf("Total Hosts: %d    Commands Succesful: %d    Commands Failed:  %d    Hosts Failed: %d", len(sshHostInfo), successResults, failedResults, failedHosts)
	log.Printf("%.2fs Overall elapsed time\n", time.Since(start).Seconds())
}
