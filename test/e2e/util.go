package e2e

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"
)

const nameSuffixLength int = 8

func createDirectory(path string) error {
	return os.Mkdir(path, 0700)
}

// IsDirEmpty checks whether a directory is empty or not.
func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// A DNS-1123 subdomain must consist of lower case alphanumeric characters
var lettersAndDigits = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		// #nosec G404: random number generator not used for security purposes
		randIdx := rand.Intn(len(lettersAndDigits))
		b[i] = lettersAndDigits[randIdx]
	}
	return string(b)
}

func randName(prefix string) string {
	return prefix + randSeq(nameSuffixLength)
}

func antctlCoverageArgs(antctlPath string) []string {
	const timeFormat = "20060102T150405Z0700"
	timeStamp := time.Now().Format(timeFormat)
	return []string{antctlPath, "-test.run=TestBincoverRunMain", fmt.Sprintf("-test.coverprofile=antctl-%s.out", timeStamp)}
}

// runAntctl runs antctl commands on antrea Pods, the controller, or agents.
func runAntctl(podName string, cmds []string, data *TestData) (string, string, error) {
	var containerName string
	var namespace string
	if strings.Contains(podName, "agent") {
		containerName = "antrea-agent"
		namespace = antreaNamespace
	} else if strings.Contains(podName, "flow-aggregator") {
		containerName = "flow-aggregator"
		namespace = flowAggregatorNamespace
	} else {
		containerName = "antrea-controller"
		namespace = antreaNamespace
	}
	stdout, stderr, err := data.RunCommandFromPod(namespace, podName, containerName, cmds)
	// remove Bincover metadata if needed
	if err == nil {
		index := strings.Index(stdout, "START_BINCOVER_METADATA")
		if index != -1 {
			stdout = stdout[:index]
		}
	}
	return stdout, stderr, err
}

// antctlOutput is a helper function for generating antctl outputs.
func antctlOutput(stdout, stderr string) string {
	return fmt.Sprintf("antctl stdout:\n%s\nantctl stderr:\n%s", stdout, stderr)
}
