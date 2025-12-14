package main

import (
        "fmt"
        "log"
        "net"
        "os"
		"io"
        "time"
		"syscall"
		"unsafe"

        "golang.org/x/crypto/ssh"
		"golang.org/x/term"
)

type winsize struct {
	Height uint16
	Width  uint16
	x      uint16
	y      uint16
}

type AppCfg struct {
        hostMapping map[string]string // { host "ip:port": auth "keyPath" }
}

func main() {
        if len(os.Args) < 2 {
                log.Fatalf("Usage: %s /<host>/<directory>", os.Args[0])
        }

        host := os.Args[1]
		directory := os.Args[2]

        // Configure SSH client
        user := "ozerova"
		secret := "0112"
        sshClientConfig := &ssh.ClientConfig{
                User: user,
                Auth: []ssh.AuthMethod{
						ssh.Password(secret),
                },
                HostKeyCallback: ssh.InsecureIgnoreHostKey(),
                Timeout:         5 * time.Second,
        }

        // Connect to the host
        port := "22"
        address := net.JoinHostPort(host, port)
        client, err := ssh.Dial("tcp", address, sshClientConfig)
        if err != nil {
                log.Fatalf("Failed to connect to %s: %s", host, err)
        }
        defer client.Close()

        fmt.Printf("Connected to %s\n", host)

        // Create an SSH session
        session, err := client.NewSession()
        if err != nil {
                log.Fatalf("failed to create session: %s", err)
        }
        defer session.Close()

		// Configure IO
		session.Stdout = os.Stdout 
		session.Stderr = os.Stderr
		in, err := session.StdinPipe()
		if err != nil {
			log.Fatalf("Failed to get StdinPipe: %v", err)
		}

        // Get remote shell
		getPTY(session)
		startInteractiveSession(session, directory, in)
}

func getPTY(session *ssh.Session) {
	w := getWinSize()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // disable echoing
		ssh.ICRNL: 		   1,	  // some weird shit
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	if err := session.RequestPty("xterm", int(w.Height), int(w.Width), modes); err != nil {
		log.Fatal("failed to get PTY: ", err)
	}
}

func startInteractiveSession(session *ssh.Session, directory string, in io.WriteCloser) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Failed to set terminal to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	command := fmt.Sprintf("cd %s && exec -l $SHELL", directory)
	if err := session.Start(command); err != nil {
		log.Fatalf("Failed to start remote command: %v", err)
	}

	go func() {
		if _, err := io.Copy(in, os.Stdin); err != nil && err != io.EOF {
			log.Printf("io.Copy error: %v", err)
		}
	}()

	if err := session.Wait(); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			log.Printf("Remote session closed with exit status %d", exitErr.ExitStatus())
		} else {
			log.Printf("Session wait error: %v", err)
		}
	}
}

func getWinSize() *winsize {
	ws := &winsize{}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if err != 0 {
		return &winsize{Height: 25, Width: 80}
	}
	return ws
}
