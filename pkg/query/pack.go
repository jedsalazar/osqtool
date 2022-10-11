package query

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"k8s.io/klog/v2"
)

type Pack struct {
	Queries   map[string]*Metadata `json:"queries,omitempty"`
	Discovery map[string]*Metadata `json:"discovery,omitempty"`

	// Refer to obj.HasMember() calls in osquery/config/packs.cpp
	Shard    int    `json:"shard,omitempty"`
	Platform string `json:"platform,omitempty"`
	Version  string `json:"version,omitempty"`
	Oncall   string `json:"oncall,omitempty"`
}

// RenderPack renders an osquery pack file from a set of queries.
func RenderPack(qs map[string]*Metadata) ([]byte, error) {
	pack := &Pack{Queries: qs}
	out, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return out, err
	}

	// hand massaging the query part for aesthetics
	out = bytes.ReplaceAll(out, []byte(`\u003e`), []byte(">"))
	out = bytes.ReplaceAll(out, []byte(`\u003c`), []byte("<"))
	return bytes.ReplaceAll(out, []byte(`\n`), []byte(" \\\n    ")), nil
}

// LoadPack loads and parses an osquery pack file.
func LoadPack(path string) (*Pack, error) {
	pack := &Pack{}

	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %v", err)
	}

	// workaround: invalid character '\n' in string escape code
	// replace trailing \<newline> with \<escaped newline>
	bs = bytes.ReplaceAll(bs, []byte("\\\n"), []byte("\\\\n"))

	// workaround: cannot unmarshal number into Go struct field Metadata.queries.interval of type string
	nakedInterval := regexp.MustCompile(`"interval"\s*:\s*(\d+),`)
	bs = nakedInterval.ReplaceAll(bs, []byte("\"interval\": \"$1\","))
	klog.Infof("bytes: %s", bs)

	err = json.Unmarshal(bs, pack)
	if err != nil {
		return nil, fmt.Errorf("unmarshal: %v", err)
	}

	// Final repairs
	for _, v := range pack.Queries {
		if pack.Platform != "" && v.Platform == "" {
			v.Platform = pack.Platform
		}
		v.Query = strings.ReplaceAll(v.Query, "\\n", "\n")
	}

	return pack, nil
}

// SaveToDirectories saves a map of queries into a directory.
func SaveToDirectory(mm map[string]*Metadata, destination string) error {
	for name, m := range mm {
		s, err := Render(m)
		if err != nil {
			return fmt.Errorf("render: %v", err)
		}

		bs := []byte(s)
		path := filepath.Join(destination, name+".sql")
		klog.Infof("Writing %d bytes to %s ...", len(bs), path)
		err = os.WriteFile(path, bs, 0o600)
		if err != nil {
			return fmt.Errorf("write file: %v", err)
		}
	}
	return nil
}