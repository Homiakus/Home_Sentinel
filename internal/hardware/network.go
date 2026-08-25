package hardware

import (
	"net"
	"strings"
)

func ProbeNetwork() []InterfaceInfo {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]InterfaceInfo, 0, len(ifs))
	for _, iface := range ifs {
		info := InterfaceInfo{Name: iface.Name, Up: iface.Flags&net.FlagUp != 0, Loopback: iface.Flags&net.FlagLoopback != 0, ContainerLike: isContainerInterface(iface.Name)}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			info.Addresses = append(info.Addresses, a.String())
		}
		out = append(out, info)
	}
	return out
}
func isContainerInterface(n string) bool {
	return n == "docker0" || strings.HasPrefix(n, "veth") || strings.HasPrefix(n, "br-") || strings.HasPrefix(n, "cni") || strings.HasPrefix(n, "flannel")
}
func DiscoveryCIDRs(ifs []InterfaceInfo) []string {
	seen := map[string]bool{}
	var out []string
	for _, i := range ifs {
		if !i.Up || i.Loopback || i.ContainerLike {
			continue
		}
		for _, raw := range i.Addresses {
			ip, netw, err := net.ParseCIDR(raw)
			if err != nil || ip == nil || !ip.IsPrivate() {
				continue
			}
			cidr := netw.String()
			if !seen[cidr] {
				seen[cidr] = true
				out = append(out, cidr)
			}
		}
	}
	return out
}
