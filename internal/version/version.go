package version

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

var (
	COMMIT_ID  = "<unknown-commit-id>"
	BUILD_TIME = "<unknown-build-time>"
	VERSION    = "<unknown-version>"
)

func GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "<unknown-hostname>"
	}
	return hostname
}

func GetUsername() string {
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	if username == "" {
		username = "<unknown-user>"
	}
	return username
}

func GetGoVersion() string {
	return strings.TrimPrefix(strings.TrimPrefix(runtime.Version(), "go"), "go")
}

func GetRuntimePlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func GetDefaultUserAgent() string {
	builder := strings.Builder{}
	builder.WriteString("curl-go/" + VERSION)
	builder.WriteString("/(" + GetRuntimePlatform() + ")")
	return builder.String()
}

func PrintVersionInfo() {
	fmt.Println("Version: " + VERSION)
	fmt.Println("CommitID: " + COMMIT_ID)
	fmt.Println("BuildTime: " + BUILD_TIME)
	fmt.Println("GoVersion: " + GetGoVersion())
	fmt.Println("DefaultUserAgent: " + GetDefaultUserAgent())
	fmt.Println("Platform: " + GetRuntimePlatform())
	fmt.Println("Hostname: " + GetHostname())
	fmt.Println("Username: " + GetUsername())
}
