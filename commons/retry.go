package commons

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

func RunWithRetry(retry int, retryInterval int) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "RunWithRetry",
	})

	var err error
	for i := 0; i < retry; i++ {
		logger.Infof("running child process for retry #%d", i)
		err = runChild()
		if err == nil {
			logger.Debug("completed child process successfully")
			return nil
		}

		logger.Errorf("%+v", err)
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)

		logger.Errorf("Waiting %d seconds for next try...", retryInterval)
		fmt.Fprintf(os.Stderr, "Waiting %d seconds for next try...", retryInterval)

		sleepTime := time.Duration(retryInterval * int(time.Second))
		time.Sleep(sleepTime)
	}

	if err != nil {
		err = xerrors.Errorf("failed to run after %d retries, job failed: %w", retry, err)
		logger.Errorf("%+v", err)
		return err
	}

	logger.Info("completed child process successfully")
	return nil
}

func runChild() error {
	envManager := GetEnvironmentManager()
	env := envManager.Environment

	configTypeIn := &ConfigTypeIn{
		Host:     env.Host,
		Port:     env.Port,
		Zone:     env.Zone,
		Username: env.Username,
		Password: envManager.Password,
	}

	configTypeInYamlBytes, err := configTypeIn.ToYAML()
	if err != nil {
		yamlErr := xerrors.Errorf("failed to get config yaml: %w", err)
		return yamlErr
	}

	bin := os.Args[0]
	args := os.Args[1:]

	newArgs := []string{}
	newArgs = append(newArgs, "--retry_child")
	newArgs = append(newArgs, args...)
	newArgs = append(newArgs, "--force")

	cmd := exec.Command(bin, newArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		pipeErr := xerrors.Errorf("failed to get stdin pipe: %w", err)
		return pipeErr
	}

	// start
	err = cmd.Start()
	if err != nil {
		cmdErr := xerrors.Errorf("failed to start the child process: %w", err)
		return cmdErr
	}

	// send config to stdin
	_, err = stdinPipe.Write(configTypeInYamlBytes)
	if err != nil {
		writeErr := xerrors.Errorf("failed to send config yaml to stdin pipe: %w", err)
		return writeErr
	}

	stdinPipe.Close()

	err = cmd.Wait()
	if err != nil {
		cmdErr := xerrors.Errorf("failed to wait the child process: %w", err)
		return cmdErr
	}
	return nil
}
