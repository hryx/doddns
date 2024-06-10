package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/digitalocean/godo"
)

type config struct {
	TokenFile     string  `json:"token_file"`
	Domain        string  `json:"domain"`
	Hostname      string  `json:"hostname"`
	PeriodSeconds float64 `json:"period"`
	IPv4          bool    `json:"ipv4"`
	IPv6          bool    `json:"ipv6"`
}

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "", `Path to config JSON file`)
	flag.Parse()
	var cfg config
	err := json.Unmarshal(mustReadFile(cfgPath), &cfg)
	if err != nil {
		log.Fatalf("unmarshal config: %v", err)
	}
	key := strings.TrimSpace(string(mustReadFile(cfg.TokenFile)))
	if key == "" {
		log.Fatal("API key is empty")
	}

	if !cfg.IPv4 && !cfg.IPv6 {
		log.Fatal("at least one must be set to true: ipv4, ipv6")
	}

	fqdn := fmt.Sprintf("%s.%s", cfg.Hostname, cfg.Domain)
	log.Printf("fully qualified domain set to %s", fqdn)

	ctx := context.Background()
	client := godo.NewFromToken(key)
	opt := &godo.ListOptions{}
	var records4, records6 []int
	for {
		recs, res, err := client.Domains.RecordsByName(ctx, cfg.Domain, fqdn, opt)
		if err != nil {
			log.Fatalf("get DNS records by domain name: %v", err)
		}
		for _, rec := range recs {
			switch {
			case cfg.IPv4 && rec.Type == "A":
				records4 = append(records4, rec.ID)
				lastIPv4 = rec.Data
			case cfg.IPv6 && rec.Type == "AAAA":
				records6 = append(records6, rec.ID)
				lastIPv6 = rec.Data
			default:
				continue
			}
			log.Printf("found %s record ID %d: %s", rec.Type, rec.ID, rec.Data)
		}
		if res.Links == nil || res.Links.IsLastPage() {
			break
		}
		page, err := res.Links.CurrentPage()
		if err != nil {
			log.Fatalf("get current page: %v", err)
		}
		opt.Page = page + 1
	}
	if len(records4) == 0 && len(records6) == 0 {
		log.Fatalf("no A or AAAA records found for %s", fqdn)
	}

	dur := time.Duration(float64(time.Second) * cfg.PeriodSeconds)
	if dur < time.Second*5 {
		log.Printf("warning: period %v too short; increasing", dur)
		dur = time.Minute * 15
	}
	log.Printf("period set to %s", dur)
	ticker := time.NewTicker(dur)
	for {
		loopMain(client, cfg.Domain, cfg.Hostname, records4, records6)
		<-ticker.C
	}
}

func mustReadFile(path string) []byte {
	if path == "" {
		log.Fatal("file path empty")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}
	return b
}

var lastIPv4, lastIPv6 string

func loopMain(client *godo.Client, domain, hostname string, records4, records6 []int) {
	updateRecords(client, updateRecordsInput{
		domain:             domain,
		hostname:           hostname,
		recordIDs:          records4,
		recordType:         "A",
		publicIPServiceURL: "https://api.ipify.org",
		lastIP:             &lastIPv4,
	})
	updateRecords(client, updateRecordsInput{
		domain:             domain,
		hostname:           hostname,
		recordIDs:          records6,
		recordType:         "AAAA",
		publicIPServiceURL: "https://api6.ipify.org",
		lastIP:             &lastIPv6,
	})
}

type updateRecordsInput struct {
	domain, hostname   string
	recordIDs          []int
	recordType         string
	lastIP             *string
	publicIPServiceURL string
}

func updateRecords(client *godo.Client, in updateRecordsInput) {
	if len(in.recordIDs) == 0 {
		return
	}
	ip, err := getPublicIP(in.publicIPServiceURL)
	if err != nil {
		log.Printf("error: get public IP from %s: %v", in.publicIPServiceURL, err)
		return
	}
	if ip == *in.lastIP {
		return
	}
	log.Printf("public IP address changed from %s to %s", *in.lastIP, ip)
	for _, recID := range in.recordIDs {
		ctx := context.Background()
		_, _, err := client.Domains.EditRecord(ctx, in.domain, recID, &godo.DomainRecordEditRequest{
			Type: in.recordType,
			Name: in.hostname,
			Data: ip,
		})
		if err != nil {
			log.Printf("error: edit record: %v", err)
			return
		}
	}
	*in.lastIP = ip
}

func getPublicIP(serviceURL string) (string, error) {
	res, err := http.Get(serviceURL)
	if err != nil {
		return "", fmt.Errorf("call IP service: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP status: %s", res.Status)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	ipStr := string(b)
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("response was not an IP address: %q", ipStr)
	}
	return ipStr, nil
}
