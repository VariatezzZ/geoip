# geoip

Builds a small Xray GeoIP database that contains only the `CN` and `PRIVATE`
categories.

## Outputs

| File | Categories |
| --- | --- |
| `geoip.dat` | `CN`, `PRIVATE` |
| `geoip-only-cn-private.dat` | `CN`, `PRIVATE` |

Both files are written from the same serialized byte slice. They are
byte-identical and only the file names differ.

## CN data

`CN` follows the `in-cn` recipe from the upstream `geoip` branch:

- IPinfo Lite `CN` IPv6 prefixes.
- IPv4 prefixes from `zhufengme/block_cn_files`.
- IPv4 prefixes from `misakaio/chnroutes2`.
- IPv4 prefixes from `gaoyifan/china-operator-ip`.
- IPv6 prefixes from `gaoyifan/china-operator-ip`.

The raw IPinfo `CN` IPv4 country data is intentionally not included. This
matches the upstream recipe, which uses IPinfo only for IPv6.

## PRIVATE data

`PRIVATE` contains IPv4 and IPv6 special-purpose, reserved, documentation,
benchmarking, and multicast ranges. The list is stored in `data/private.txt`.

## IPinfo database

The build uses two third-party daily mirrors:

| Role | Repository | Check |
| --- | --- | --- |
| Primary | `Alice39s/ipinfo-csv-lite` | SHA-256 from `checksums.txt`, then MMDB open |
| Fallback | `NetworkCats/IPinfoLite-Download` | MMDB open |

No IPinfo token is required. There is no data freshness check. If the primary
mirror fails download, checksum, or MMDB validation, the fallback is used. The
build fails only when both mirrors fail.

## Local build

Requirements: Go 1.26 or newer.

```sh
go test ./...
go vet ./...
go run ./cmd/fetch-ipinfo -config config.yaml
go run . -config config.yaml
```

The downloader writes `./ipinfo/ipinfo_lite.mmdb`. The builder reads that local
file, downloads the CN text sources, aggregates all prefixes, and writes both
DAT files under `./output/`.

Verify the result:

```sh
cmp ./output/geoip.dat ./output/geoip-only-cn-private.dat
sha256sum ./output/geoip.dat ./output/geoip-only-cn-private.dat
```

Relative paths in `config.yaml` are resolved from the directory that contains
the configuration file.

## Automation

The repository uses one branch, `main`.

- A push to `main` runs tests, vetting, and a full build.
- A manual dispatch builds and publishes a GitHub Release.
- Releases contain both DAT files and their SHA-256 sums.
- Release creation and cleanup use the GitHub CLI directly, without a third-party release action.
- Workflow actions use Node.js 24 compatible majors: `checkout@v6` and `setup-go@v6`.
- Only the five newest releases and their tags are retained.
- Release tags use the UTC build timestamp. No release branch is created.

Stable download URLs:

```text
https://github.com/VariatezzZ/geoip/releases/latest/download/geoip.dat
https://github.com/VariatezzZ/geoip/releases/latest/download/geoip-only-cn-private.dat
```

## Data attribution

IP address data is powered by IPinfo (https://ipinfo.io/lite).

Additional CN data sources:

- https://github.com/zhufengme/block_cn_files
- https://github.com/misakaio/chnroutes2
- https://github.com/gaoyifan/china-operator-ip

IPinfo Lite mirrors:

- https://github.com/Alice39s/ipinfo-csv-lite
- https://github.com/NetworkCats/IPinfoLite-Download
