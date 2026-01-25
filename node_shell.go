package proxmox

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type NodeShell struct {
	node    *Node
	send    chan string
	recv    chan string
	errs    chan error
	close   func() error
	context context.Context
}

func (n *Node) Shell(ctx context.Context) NodeShell {
	return NodeShell{node: n, context: ctx}
}
func (s *NodeShell) Open() error {
	vnc, err := s.node.TermProxy(s.context)
	if err != nil {
		return err
	}

	s.send, s.recv, s.errs, s.close, err = s.node.VNCWebSocket(vnc)

	if err != nil {
		return err
	}

	err = s.LoginIfRequired()
	return err
}

func (s *NodeShell) ReadLines() (chan string, func()) {
	lines := make(chan string, 10)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case msg := <-s.recv:
				if msg != "" {
					msgLines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(msg, "\r\n", "\n"), "\r", "\n"), "\n")
					for _, line := range msgLines {
						lines <- line
					}
				}
			case err := <-s.errs:
				if err != nil {
					fmt.Println("ERROR: " + err.Error())
					return
				}
			}

		}
	}()

	stop := func() {
		close(done)
	}

	return lines, stop
}

func (s *NodeShell) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	seed := rand.NewSource(time.Now().UnixNano())
	random := rand.New(seed)

	result := make([]byte, length)
	for i := range result {
		result[i] = charset[random.Intn(len(charset))]
	}
	return string(result)
}

func (s *NodeShell) ExecuteCommand(command string) (error, int, string, string) {
	return s.executeCommand(command, true)
}

func (s *NodeShell) ExecuteCommandMaybeFailing(command string) (error, int, string, string) {
	return s.executeCommand(command, false)
}

func (s *NodeShell) executeCommand(command string, raiseErrorOnNonZeroExitCode bool) (error, int, string, string) {
	commandTimeout := time.Duration(60) * time.Second
	sentinel := s.generateRandomString(20)
	stdoutPrefix := s.generateRandomString(4) + ">"
	stderrPrefix := s.generateRandomString(4) + ">"

	//s.send <- "set -o pipefail"
	cmd := fmt.Sprintf(`%s 1> >(sed "s/^/%s/") 2> >(sed "s/^/%s/" >&2)`, command, stdoutPrefix, stderrPrefix)
	//s.send <- fmt.Sprintf("(%s | while read line; do echo \"%s$line\"; done) 2> (while read line; do echo \"%s$line\"; done)", command, stdoutPrefix, stderrPrefix)
	s.send <- cmd

	time.Sleep(1 * time.Second)
	s.send <- fmt.Sprintf(`echo "%s=$?"`, sentinel)

	line, allLines, err := s.ExpectMessageRegex(regexp.MustCompile(fmt.Sprintf(`^%s=\d+$`, sentinel)), &commandTimeout)

	var stdoutLines []string
	var stderrLines []string
	var stdout string
	var stderr string
	if allLines != nil {
		for _, item := range *allLines {
			if strings.Contains(item, cmd) {
				continue
			}
			if start := strings.Index(item, stdoutPrefix); start >= 0 {
				stdoutLines = append(stdoutLines, item[start+len(stdoutPrefix):])
			}
			if start := strings.Index(item, stderrPrefix); start >= 0 {
				stderrLines = append(stderrLines, item[start+len(stderrPrefix):])
			}
		}
	}

	if stdoutLines != nil {
		stdout = strings.Join(stdoutLines, "\n")
	}
	if stderrLines != nil {
		stderr = strings.Join(stderrLines, "\n")
	}

	if err != nil {
		return err, 0, stdout, stderr
	}

	returnCodeStr := strings.TrimPrefix(*line, sentinel+"=")
	returnCode, err := strconv.Atoi(returnCodeStr)
	if err != nil {
		return err, 0, stdout, stderr
	}

	if raiseErrorOnNonZeroExitCode && returnCode != 0 {
		return fmt.Errorf("command failed with code %d", returnCode), 0, stdout, stderr
	}

	return nil, returnCode, stdout, stderr
}

func (s *NodeShell) expectMessage(matcher func(line string) bool, timeout *time.Duration) (*string, *[]string, error) {
	var timerTimeout time.Duration
	if timeout != nil {
		timerTimeout = *timeout
	} else {
		timerTimeout = time.Duration(10) * time.Second
	}
	lines, stop := s.ReadLines()
	defer stop()

	readLines := make([]string, 10)
	for {
		select {

		case line, open := <-lines:
			if !open {
				return nil, nil, errors.New("No new lines")
			}
			readLines = append(readLines, line)
			if matcher(line) {
				return &line, &readLines, nil
			}
		case <-time.After(timerTimeout):
			return nil, nil, errors.New("timeout while waiting for expected message")
		}

	}
}

func (s *NodeShell) ExpectMessageExact(message string, timeout *time.Duration) (*string, *[]string, error) {
	return s.expectMessage(func(line string) bool {
		return line == message
	}, timeout)
}

func (s *NodeShell) ExpectMessageContains(message string, timeout *time.Duration) (*string, *[]string, error) {
	return s.expectMessage(func(line string) bool {
		return strings.Contains(line, message)
	}, timeout)
}

func (s *NodeShell) ExpectMessageRegex(re *regexp.Regexp, timeout *time.Duration) (*string, *[]string, error) {
	return s.expectMessage(func(line string) bool {
		return re.MatchString(line)
	}, timeout)
}

func (s *NodeShell) LoginIfRequired() error {
	if s.node.client.credentials == nil {
		return errors.New("shell session requires credentials")
	}

	if s.node.client.credentials.Username == "root@pam" {
		return nil
	}

	if !strings.HasSuffix(s.node.client.credentials.Username, "@pam") {
		return errors.New("shell session requires pam user")
	}

	username := strings.TrimSuffix(s.node.client.credentials.Username, "@pam")
	password := s.node.client.credentials.Password
	defaultTimeout := time.Duration(5) * time.Second

	// linux login prompt
	if _, _, err := s.ExpectMessageContains("login", &defaultTimeout); err != nil {
		return err
	}

	if err := s.WriteMessage(fmt.Sprintf("%s", username)); err != nil {
		return err
	}

	// password prompt for user
	if _, _, err := s.ExpectMessageContains(username, &defaultTimeout); err != nil {
		return err
	}
	if _, _, err := s.ExpectMessageContains("Password", &defaultTimeout); err != nil {
		return err
	}
	if err := s.WriteMessage(fmt.Sprintf("%s", password)); err != nil {
		return err
	}

	// successful login - check for either "Last login" (SSH) or shell prompt (node shell)
	// Shell prompt patterns vary widely across distros and configurations, so we check for:
	// 1. Lines ending with $ or # (common prompt endings)
	// 2. Lines containing user@host: pattern (common bash/zsh format)
	// 3. Lines ending with ] followed by $ or # (bracket-style prompts)

	// Match lines ending with $ or # with optional whitespace
	// Captures: "user@host:~$ ", "$ ", "[user@host dir]# ", etc.
	promptEndPattern := regexp.MustCompile(`[$#]\s*$`)

	// Match user@host: pattern (common in bash/zsh)
	userHostPattern := regexp.MustCompile(`[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+:`)

	lines, stop := s.ReadLines()
	defer stop()

	loginSuccessTimeout := time.After(defaultTimeout)
	for {
		select {
		case line, open := <-lines:
			if !open {
				return errors.New("connection closed while waiting for login confirmation")
			}

			// Check for "Last login" message (typical in SSH)
			if strings.Contains(line, "Last login") {
				return nil
			}

			// Check for shell prompt indicators
			// A line is likely a prompt if it ends with $ or # AND contains user@host pattern
			// OR if it's a simple $ or # prompt
			trimmedLine := strings.TrimSpace(line)
			hasPromptEnd := promptEndPattern.MatchString(trimmedLine)
			hasUserHost := userHostPattern.MatchString(trimmedLine)

			if hasPromptEnd && (hasUserHost || trimmedLine == "$" || trimmedLine == "#") {
				return nil
			}

			// Check for login failure indicators
			if strings.Contains(line, "Login incorrect") || strings.Contains(line, "Authentication failure") {
				return errors.New("login failed: incorrect credentials")
			}
		case <-loginSuccessTimeout:
			return errors.New("timeout while waiting for login confirmation")
		}
	}
}

func (s *NodeShell) WriteMessage(message string) error {
	s.send <- message
	return nil
}

func (s *NodeShell) Close() error {
	return s.close()
}
