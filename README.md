# doddns.go

Script that recurringly checks the host's public IP address and updates [DigitalOcean](https://www.digitalocean.com) DNS records.

Public IP address is obtained using the free public service [ipify](https://www.ipify.org).

Updates both A and AAAA records. All records of the same type will be set to the same IP address.
You must create your DigitalOcean DNS domain and domain record(s) ahead of time.
This script will only update existing records, not create new ones.

## Configuration

Must be a JSON file on disk. Supply the config path with `--config path/to/conf.json`.

Options:

- `token_file`: Path to file on disk containing only your DigitalOcean personal access token.
- `domain`: The domain managed in DigitalOcean.
- `hostname`: The hostname or subdomain that should point to this host.
- `period`: Update period, specified in seconds.
- `ipv4` and `ipv6`: One or both of these must be set to true. Enables updating A and AAAA records respectively.

Example:

```json
{
    "token_file": "/home/me/my_doddns_token",
    "domain": "example.com",
    "subdomain": "rhombus",
    "period": 600,
    "ipv4": true,
    "ipv6": false
}
```
