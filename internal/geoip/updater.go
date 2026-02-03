package geoip

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SettingsStore interface {
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
}

type Updater struct {
	mu        sync.Mutex
	store     SettingsStore
	resolver  *Resolver
	geoDBPath string
	client    *http.Client
}

func NewUpdater(store SettingsStore, resolver *Resolver, geoDBPath string) *Updater {
	return &Updater{
		store:     store,
		resolver:  resolver,
		geoDBPath: geoDBPath,
		client:    &http.Client{Timeout: 2 * time.Minute},
	}
}

func (u *Updater) DBPath() string {
	return u.geoDBPath
}

func (u *Updater) Download() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	key, err := u.store.GetSetting("maxmind.license_key")
	if err != nil {
		return fmt.Errorf("getting license key: %w", err)
	}
	if key == "" {
		return nil
	}

	dlURL := fmt.Sprintf(
		"https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=%s&suffix=tar.gz",
		url.QueryEscape(key),
	)

	resp, err := u.client.Get(dlURL)
	if err != nil {
		return fmt.Errorf("downloading GeoLite2: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MaxMind returned status %d", resp.StatusCode)
	}

	destDir := filepath.Dir(u.geoDBPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating geoip dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(destDir, "geolite2-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, io.LimitReader(resp.Body, 100<<20)); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing tar.gz: %w", err)
	}
	tmpFile.Close()

	mmdbPath, err := extractMMDB(tmpPath, destDir)
	if err != nil {
		return fmt.Errorf("extracting mmdb: %w", err)
	}
	defer os.Remove(mmdbPath)

	if err := os.Rename(mmdbPath, u.geoDBPath); err != nil {
		return fmt.Errorf("moving mmdb: %w", err)
	}

	if err := u.resolver.Reload(u.geoDBPath); err != nil {
		return fmt.Errorf("reloading resolver: %w", err)
	}

	_ = u.store.SetSetting("maxmind.last_updated", time.Now().UTC().Format(time.RFC3339))
	log.Println("geoip: database updated successfully")
	return nil
}

func extractMMDB(tarGzPath, destDir string) (string, error) {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if strings.HasSuffix(hdr.Name, ".mmdb") {
			tmpOut, err := os.CreateTemp(destDir, "geolite2-*.mmdb")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmpOut, io.LimitReader(tr, 100<<20)); err != nil {
				tmpOut.Close()
				os.Remove(tmpOut.Name())
				return "", err
			}
			tmpOut.Close()
			return tmpOut.Name(), nil
		}
	}
	return "", fmt.Errorf("no .mmdb file found in archive")
}

func (u *Updater) Start(ctx context.Context) {
	if err := u.Download(); err != nil {
		log.Printf("geoip: initial download: %v", err)
	}

	ticker := time.NewTicker(7 * 24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := u.Download(); err != nil {
				log.Printf("geoip: scheduled download: %v", err)
			}
		}
	}
}
