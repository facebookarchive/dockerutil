package dockerutil

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/facebookgo/stackerr"
	"github.com/samalba/dockerclient"
)

// BindingAddr provides the address for the container and binding.
func BindingAddr(d dockerclient.Client, name, binding string) (string, error) {
	ci, err := d.InspectContainer(name)
	if err != nil {
		return "", stackerr.Wrap(err)
	}

	ip, err := dockerIP(d)
	if err != nil {
		return "", err
	}

	hostname, err := etcHostsName(ip)
	if err != nil {
		return "", err
	}

	if hostname == "" {
		hostname = ip.String()
	}

	addr := fmt.Sprintf(
		"%s:%s",
		hostname,
		ci.NetworkSettings.Ports[binding][0].HostPort,
	)
	return addr, nil
}

func dockerIP(d dockerclient.Client) (net.IP, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("boot2docker", "ip").Output()
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		ip := net.ParseIP(string(out))
		if ip == nil {
			return nil, stackerr.Newf("invalid ip from boot2docker: %s", out)
		}
		return ip, nil
	case "linux":
		return net.IPv4zero, nil
	default:
		return nil, stackerr.New("dont know how to get docker IP")
	}
}

// if /etc/hosts contains an entry for the given IP it will be returned. this
// allows for a pretty name to be used for the dockerIP if available. if the ip
// is not found an empty string and a nil error will be returned.
func etcHostsName(ip net.IP) (string, error) {
	f, err := os.Open("/etc/hosts")
	if err != nil {
		return "", stackerr.Wrap(err)
	}
	defer f.Close()

	ips := ip.String()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, ips) {
			return strings.Fields(text)[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", stackerr.Wrap(err)
	}

	// not found
	return "", nil
}
