package ssh

import (
	"fmt"
	"strings"
)

func Config(repoNames []string) string {
	var result strings.Builder

	for _, repoName := range repoNames {
		result.WriteString(fmt.Sprintf("Host github.com-%s\n", repoName))
		result.WriteString(fmt.Sprintf("    HostName github.com\n"))
		result.WriteString(fmt.Sprintf("    User git\n"))
		result.WriteString(fmt.Sprintf("    IdentityFile ~/.ssh/%s/identity\n", repoName))
		result.WriteString(fmt.Sprintf("\n"))
	}

	return result.String()
}
