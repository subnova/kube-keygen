package ssh

import (
	"os/exec"
)

func KeyScan(domains []string) (knownHosts string, err error) {
	out, err := exec.Command("ssh-keyscan", domains...).Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}
