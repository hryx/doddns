package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	Subdomain     string  `json:"subdomain"`
	PeriodSeconds float64 `json:"period"`
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

	client := godo.NewFromToken(key)
	ctx := context.Background()
	fqdn := fmt.Sprintf("%s.%s", cfg.Subdomain, cfg.Domain)
	recs, _, err := client.Domains.RecordsByTypeAndName(ctx, cfg.Domain, "A", fqdn, nil)
	if err != nil {
		log.Fatalf("get DNS records by domain name: %v", err)
	}
	if len(recs) == 0 {
		log.Fatalf("no A record found for %s", fqdn)
	}
	recID := recs[0].ID
	log.Printf("found record %d for %s", recID, fqdn)

	dur := time.Duration(float64(time.Second) * cfg.PeriodSeconds)
	if dur < time.Second*5 {
		log.Printf("warning: period %v too short", dur)
		dur = time.Minute * 15
	}
	log.Printf("period set to %s", dur)
	ticker := time.NewTicker(dur)
	for {
		loopMain(client, cfg.Domain, cfg.Subdomain, recID)
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

var lastKnownIP string

func loopMain(client *godo.Client, domain, subdomain string, recID int) {
	ip, err := getPublicIP()
	if err != nil {
		log.Printf("error: get public IP address: %v", err)
		return
	}
	if ip == lastKnownIP {
		return
	}
	if lastKnownIP == "" {
		log.Printf("public IP address: %s", ip)
	} else {
		log.Printf("public IP address changed from %s to %s", lastKnownIP, ip)
	}
	lastKnownIP = ip

	ctx := context.Background()
	_, _, err = client.Domains.EditRecord(ctx, domain, recID, &godo.DomainRecordEditRequest{
		Type: "A",
		Name: subdomain,
		Data: ip,
	})
	if err != nil {
		log.Printf("error: edit record: %v", err)
		return
	}
}

func getPublicIP() (string, error) {
	res, err := http.Get("http://ip-api.com/json?fields=query")
	if err != nil {
		return "", fmt.Errorf("call IP service: %w", err)
	}
	defer res.Body.Close()
	type ipRes struct {
		Query string
	}
	var ir ipRes
	err = json.NewDecoder(res.Body).Decode(&ir)
	if err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	ip := net.ParseIP(ir.Query)
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("response did not have an IPv4 address: %q", ir.Query)
	}
	return ir.Query, nil
}
