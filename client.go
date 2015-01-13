package dockerutil

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/facebookgo/runcmd"
	"github.com/facebookgo/stackerr"
	"github.com/samalba/dockerclient"
)

// Boot2Docker returns a DockerClient if possible configured according to
// boot2docker.
func Boot2DockerClient() (*dockerclient.DockerClient, error) {
	cmd := exec.Command("boot2docker", "shellinit")
	cmd.Env = boot2dockerEnv()
	streams, err := runcmd.Run(cmd)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}

	var host, tls, certPath string
	const (
		prefixHost     = "export DOCKER_HOST="
		prefixTLS      = "export DOCKER_TLS_VERIFY="
		prefixCertPath = "export DOCKER_CERT_PATH="
	)
	for _, lineB := range bytes.Split(streams.Stdout().Bytes(), []byte("\n")) {
		line := string(bytes.TrimSpace(lineB))
		if strings.HasPrefix(line, prefixHost) {
			host = line[len(prefixHost):]
			continue
		}
		if strings.HasPrefix(line, prefixTLS) {
			tls = line[len(prefixTLS):]
			continue
		}
		if strings.HasPrefix(line, prefixCertPath) {
			certPath = line[len(prefixCertPath):]
			continue
		}
	}

	if tls == "1" {
		return DockerWithTLS(host, certPath)
	}

	client, err := dockerclient.NewDockerClient(host, nil)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return client, nil
}

// DockerWithTLS returns a DockerClient with the certs in the specified
// directory. The names of the certs are the standard names of "cert.pem",
// "key.pem" and "ca.pem".
func DockerWithTLS(url, certPath string) (*dockerclient.DockerClient, error) {
	var tlsConfig *tls.Config
	clientCert, err := tls.LoadX509KeyPair(
		filepath.Join(certPath, "cert.pem"),
		filepath.Join(certPath, "key.pem"),
	)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}

	rootCAs := x509.NewCertPool()
	caCert, err := ioutil.ReadFile(filepath.Join(certPath, "ca.pem"))
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	rootCAs.AppendCertsFromPEM(caCert)

	tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      rootCAs,
	}
	client, err := dockerclient.NewDockerClient(url, tlsConfig)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return client, nil
}

// BestEfforDockerClient creates a docker client from one of:
//
// 1. Environment variables as defined in
//    https://docs.docker.com/reference/commandline/cli/. Specifically
//    DOCKER_HOST, DOCKER_TLS_VERIFY & DOCKER_CERT_PATH.
//
// 2. bootdocker, if darwin.
//
// 3. /run/docker.sock, if it exists.
//
// 4. /var/run/docker.sock, if it exists.
func BestEffortDockerClient() (*dockerclient.DockerClient, error) {
	host := os.Getenv("DOCKER_HOST")

	if host == "" {
		if runtime.GOOS == "darwin" {
			return Boot2DockerClient()
		}

		socketLocations := []string{"/run/docker.sock", "/var/run/docker.sock"}
		for _, l := range socketLocations {
			if _, err := os.Stat(l); err == nil {
				c, err := dockerclient.NewDockerClient(fmt.Sprintf("unix://%s", l), nil)
				if err != nil {
					return nil, stackerr.Wrap(err)
				}
				return c, nil
			}
		}

		return nil, stackerr.New("docker not configured")
	}

	if os.Getenv("DOCKER_TLS_VERIFY") != "" {
		return DockerWithTLS(host, os.Getenv("DOCKER_CERT_PATH"))
	}

	c, err := dockerclient.NewDockerClient(host, nil)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}

	return c, nil
}

// boot2dockerEnv returns a small fixed part of the environment. this ensures we're
// not affected by environment variables when we run boot2docker. for example
// `boot2docker shellinit` wont print environment variables if they're already
// set correctly.
func boot2dockerEnv() []string {
	var env []string
	for _, l := range os.Environ() {
		if boot2dockerIncludeEnv(l) {
			env = append(env, l)
		}
	}
	return env
}

var boot2dockerIncludeKeys = []string{
	"HOME=",
	"LANG=",
	"PATH=",
	"TMPDIR=",
	"USER=",
}

func boot2dockerIncludeEnv(l string) bool {
	for _, k := range boot2dockerIncludeKeys {
		if strings.HasPrefix(l, k) {
			return true
		}
	}
	return false
}
