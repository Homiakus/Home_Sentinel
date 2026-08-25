package discovery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
)

const multicast = "239.255.255.250:3702"

type Candidate struct {
	EndpointReference string   `json:"endpoint_reference"`
	XAddrs            []string `json:"xaddrs"`
	Scopes            []string `json:"scopes"`
	Types             []string `json:"types"`
	SourceIP          string   `json:"source_ip"`
}
type probeEnvelope struct {
	Body struct {
		ProbeMatches struct {
			Matches []struct {
				Endpoint struct {
					Address string `xml:"Address"`
				} `xml:"EndpointReference"`
				Types  string `xml:"Types"`
				Scopes string `xml:"Scopes"`
				XAddrs string `xml:"XAddrs"`
			} `xml:"ProbeMatch"`
		} `xml:"ProbeMatches"`
	} `xml:"Body"`
}

func ParseResponse(body []byte, sourceIP string) ([]Candidate, error) {
	var e probeEnvelope
	if err := xml.Unmarshal(body, &e); err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(e.Body.ProbeMatches.Matches))
	for _, m := range e.Body.ProbeMatches.Matches {
		c := Candidate{EndpointReference: strings.TrimSpace(m.Endpoint.Address), XAddrs: strings.Fields(m.XAddrs), Scopes: strings.Fields(m.Scopes), Types: strings.Fields(m.Types), SourceIP: sourceIP}
		if len(c.XAddrs) > 0 {
			out = append(out, c)
		}
	}
	return out, nil
}
func Scan(ctx context.Context, allowed []netip.Prefix, duration time.Duration) ([]Candidate, error) {
	if duration <= 0 {
		duration = 3 * time.Second
	}
	dst, err := net.ResolveUDPAddr("udp4", multicast)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	msg, err := probeMessage()
	if err != nil {
		return nil, err
	}
	if _, err := conn.WriteToUDP(msg, dst); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(duration)
	_ = conn.SetReadDeadline(deadline)
	seen := map[string]bool{}
	var out []Candidate
	buf := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				break
			}
			return out, err
		}
		addr, ok := netip.AddrFromSlice(src.IP)
		if !ok || !permitted(addr, allowed) {
			continue
		}
		cands, err := ParseResponse(buf[:n], addr.String())
		if err != nil {
			continue
		}
		for _, c := range cands {
			key := c.EndpointReference + "|" + strings.Join(c.XAddrs, ",")
			if !seen[key] {
				seen[key] = true
				out = append(out, c)
			}
		}
	}
	return out, nil
}
func permitted(ip netip.Addr, allowed []netip.Prefix) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, p := range allowed {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
func probeMessage() ([]byte, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	uuid := hex.EncodeToString(id)
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope" xmlns:w="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery" xmlns:dn="http://www.onvif.org/ver10/network/wsdl"><e:Header><w:MessageID>urn:uuid:%s</w:MessageID><w:To e:mustUnderstand="true">urn:schemas-xmlsoap-org:ws:2005:04:discovery</w:To><w:Action e:mustUnderstand="true">http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</w:Action></e:Header><e:Body><d:Probe><d:Types>dn:NetworkVideoTransmitter</d:Types></d:Probe></e:Body></e:Envelope>`, uuid)
	if len(body) == 0 {
		return nil, errors.New("empty probe")
	}
	return []byte(body), nil
}
