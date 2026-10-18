package appstore

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	cookiejar "github.com/juju/persistent-cookiejar"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	"github.com/majd/ipatool/v2/pkg/util/operatingsystem"
)

type Client struct {
	jar  *cookiejar.Jar
	http *http.Client
}

func New(cookiesFile string) (*Client, error) {
	if cookiesFile != "" {
		if err := os.MkdirAll(filepath.Dir(cookiesFile), 0o755); err != nil {
			return nil, err
		}
	}

	jar, err := cookiejar.New(&cookiejar.Options{Filename: cookiesFile})
	if err != nil {
		return nil, err
	}

	hc := &http.Client{Jar: jar}

	return &Client{jar: jar, http: hc}, nil
}

// hostMachine identifies this machine to Apple, the same way for sign-in
// (through ipatool) and for every other request. It skips placeholder and
// randomized MAC addresses such as 02:00:00:00:00:00, which would give many
// machines the same identity.
var hostMachine = machine.New(machine.Args{OS: operatingsystem.New()})

// guid returns the Configurator-shaped GUID: uppercase MAC address, no colons.
func guid() (string, error) {
	mac, err := hostMachine.MacAddress()
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(strings.ToUpper(mac), ":", ""), nil
}
