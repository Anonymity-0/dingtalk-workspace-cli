//go:build linux || darwin

package upgrade

import (
	"encoding/hex"

	"golang.org/x/sys/unix"
)

const skillPublicationXattr = "user.dws.skill-pub"

// markSkillPublication stamps the staged inode with a unique xattr. The
// fingerprint hasher ignores xattrs, so the mark does not change content
// identity. After a same-inode rename the dest still carries the mark; a
// concurrent replacement that recycled the inode number does not.
func markSkillPublication(path string) (string, bool) {
	raw := make([]byte, 16)
	if _, err := skillPathRandomRead(raw); err != nil {
		return "", false
	}
	token := hex.EncodeToString(raw)
	if err := unix.Lsetxattr(path, skillPublicationXattr, []byte(token), 0); err != nil {
		return "", false
	}
	return token, true
}

func skillPublicationHasMark(path, token string) bool {
	if token == "" {
		return false
	}
	buf := make([]byte, 64)
	n, err := unix.Lgetxattr(path, skillPublicationXattr, buf)
	if err != nil || n != len(token) {
		return false
	}
	return string(buf[:n]) == token
}
