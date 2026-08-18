//go:build !linux && !darwin

package upgrade

func markSkillPublication(string) (string, bool) { return "", false }

func skillPublicationHasMark(string, string) bool { return false }
