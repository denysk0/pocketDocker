package cli

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/denysk0/pocketDocker/internal/store"
	"github.com/spf13/cobra"
)

var PullCmd = &cobra.Command{
	Use:   "pull",
	Short: "pull image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("tar"); err != nil {
			return fmt.Errorf("tar command not found in PATH")
		}
		src := args[0]
		home, _ := os.UserHomeDir()
		cacheDir := filepath.Join(home, ".pocket-docker", "images")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return err
		}

		var localFile string
		var tmpFile *os.File
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			resp, err := http.Get(src)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("download failed: %s", resp.Status)
			}
			tmp, err := os.CreateTemp("", "pocketpull-*.tar")
			if err != nil {
				return err
			}
			tmpFile = tmp
			if _, err := io.Copy(tmp, resp.Body); err != nil {
				return err
			}
			if _, err := tmp.Seek(0, 0); err != nil {
				return err
			}
			if shaSum != "" {
				h := sha256.New()
				if _, err := io.Copy(h, tmp); err != nil {
					return err
				}
				if fmt.Sprintf("%x", h.Sum(nil)) != shaSum {
					return fmt.Errorf("sha256 mismatch")
				}
				if _, err := tmp.Seek(0, 0); err != nil {
					return err
				}
			}
			localFile = tmp.Name()
		} else {
			localFile = src
			if shaSum != "" {
				f, err := os.Open(localFile)
				if err != nil {
					return err
				}
				h := sha256.New()
				if _, err := io.Copy(h, f); err != nil {
					f.Close()
					return err
				}
				f.Close()
				if fmt.Sprintf("%x", h.Sum(nil)) != shaSum {
					return fmt.Errorf("sha256 mismatch")
				}
			}
		}

		name := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		destDir := filepath.Join(cacheDir, name)
		if _, err := os.Stat(destDir); err == nil {
			fmt.Println("already up-to-date")
			return nil
		}
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}
		tarCmd := exec.Command("tar", "-xf", localFile, "-C", destDir)
		tarCmd.Stdout = os.Stdout
		tarCmd.Stderr = os.Stderr
		if err := tarCmd.Run(); err != nil {
			return err
		}
		if tmpFile != nil {
			os.Remove(tmpFile.Name())
		}
		if st := getStore(); st != nil {
			info := store.ImageInfo{Name: name, Path: destDir, CreatedAt: time.Now()}
			if err := st.SaveImage(info); err != nil {
				return err
			}
		}
		return nil
	},
}

var shaSum string

func init() {
	PullCmd.Flags().StringVar(&shaSum, "sha256", "", "expected sha256 sum")
}
