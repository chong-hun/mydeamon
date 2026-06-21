package app

import (
	"errors"
	"fmt"
	"os"
	"net/http"
	"strings"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/state"
)

func (a *App) Status() (string, error) {
	target, err := a.healthTarget()
	if err != nil {
		return "", err
	}
	if healthAlive(target) {
		return "running", nil
	}
	_, ok, err := state.ReadPID(state.PIDPath(a.cfg.StateDir))
	if err != nil {
		return "", err
	}
	if ok {
		return "stale pid file", nil
	}
	return "stopped", nil
}

func (a *App) Stop() error {
	target, err := a.healthTarget()
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, "http://"+target+"/shutdown", nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		_ = state.RemovePID(state.PIDPath(a.cfg.StateDir))
		_ = os.Remove(healthAddressPath(a.cfg.StateDir))
		return nil
	}
	if resp != nil {
		statusCode := resp.StatusCode
		_ = resp.Body.Close()
		if err == nil {
			return fmt.Errorf("shutdown returned status %d", statusCode)
		}
	}
	_, ok, readErr := state.ReadPID(state.PIDPath(a.cfg.StateDir))
	if readErr != nil {
		return readErr
	}
	if err != nil {
		if ok {
			return err
		}
		return err
	}
	return errors.New("daemon is not running")
}

func (a *App) healthTarget() (string, error) {
	if addr := a.healthAddress(); addr != "" {
		return addr, nil
	}

	data, err := os.ReadFile(healthAddressPath(a.cfg.StateDir))
	if err == nil {
		if addr := strings.TrimSpace(string(data)); addr != "" {
			return addr, nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return a.cfg.Address, nil
}

func healthAlive(addr string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://" + addr + "/health")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
