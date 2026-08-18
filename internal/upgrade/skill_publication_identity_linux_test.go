//go:build linux

package upgrade

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestCrossPlatformCoverageSkillPublicationLinuxFileIdentityEdges(t *testing.T) {
	t.Run("statx without birth time is not an identity", func(t *testing.T) {
		// CI filesystems always report STATX_BTIME, so the unprovable-identity
		// path is driven through the seam: a kernel or filesystem without
		// birth-time support must yield an empty file ID, which callers treat
		// as "identity cannot be proven".
		testseam.Swap(t, &skillPathStatx, func(_ string, stx *unix.Statx_t) error {
			stx.Mask = unix.STATX_BASIC_STATS
			return nil
		})
		if got := skillPathFileIdentityImpl(t.TempDir()); got != "" {
			t.Fatalf("statx without btime = %q, want empty", got)
		}
	})

	t.Run("statx failure is not an identity", func(t *testing.T) {
		testseam.Swap(t, &skillPathStatx, func(string, *unix.Statx_t) error { return errors.New("statx denied") })
		if got := skillPathFileIdentityImpl(t.TempDir()); got != "" {
			t.Fatalf("failed statx = %q, want empty", got)
		}
	})
}
