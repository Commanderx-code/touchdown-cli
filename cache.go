package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const maxResponseBytes = 8 << 20

func cachePath(dir, u string) string {
	sum := sha256.Sum256([]byte(u))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".json")
}

func readCached(dir, u string, out any) (time.Time, error) {
	if dir == "" {
		return time.Time{}, os.ErrNotExist
	}
	path := cachePath(dir, u)
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	if len(data) > maxResponseBytes {
		return time.Time{}, fmt.Errorf("cached response too large")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return time.Time{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func writeCached(dir, u string, data []byte) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".touchdown-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), cachePath(dir, u))
}

func (c *client) getURL(ctx context.Context, u string, out any) (time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("User-Agent", "touchdown/0.3")
	resp, fetchErr := c.http.Do(req)
	if fetchErr == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
			if err == nil && len(data) > maxResponseBytes {
				err = errors.New("response too large")
			}
			if err == nil && !json.Valid(data) {
				err = errors.New("invalid JSON response")
			}
			if err == nil {
				err = json.Unmarshal(data, out)
			}
			if err == nil {
				_ = writeCached(c.cacheDir, u, data)
				return time.Time{}, nil
			}
			fetchErr = err
		} else {
			fetchErr = fmt.Errorf("ESPN returned %s", resp.Status)
		}
	}
	if cachedAt, err := readCached(c.cacheDir, u, out); err == nil {
		return cachedAt, nil
	}
	return time.Time{}, fetchErr
}
